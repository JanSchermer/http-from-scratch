package http1

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"strconv"
	"strings"
)

func ParseRequest(reader *bufio.Reader) (*http.Request, error) {
	request := &http.Request{}

	method, path, version, err := parseMethodPathVersion(reader)
	if err != nil {
		return nil, err
	}
	if version == "" {
		return nil, nil // Client disconnected
	}
	request.Method = method
	request.Proto = version
	versionStrs := strings.Split(version, "/")
	if len(versionStrs) != 2 || versionStrs[0] != "HTTP" {
		return nil, fmt.Errorf("invalid version string: %s", version)
	}
	versionNums := strings.Split(versionStrs[1], ".")
	if len(versionNums) != 2 {
		return nil, fmt.Errorf("invalid version string: %s", version)
	}
	request.ProtoMajor, err = strconv.Atoi(versionNums[0])
	if err != nil {
		return nil, err
	}
	request.ProtoMinor, err = strconv.Atoi(versionNums[1])
	if err != nil {
		return nil, err
	}
	request.RequestURI = path

	header, err := parseHeaders(reader)
	if err != nil {
		return nil, err
	}
	request.Header = header

	if len(header["Host"]) != 1 {
		return nil, fmt.Errorf("not exactly one host header")
	}
	request.Host = header["Host"][0]

	request.ContentLength = 0
	for i := range len(header["Content-Length"]) {
		length, err := strconv.ParseInt(header["Content-Length"][i], 10, 64)
		if err != nil || request.ContentLength < 0 {
			return nil, fmt.Errorf("invalid Content-Length")
		}
		if i == 0 {
			request.ContentLength = length
		} else if request.ContentLength != length {
			return nil, fmt.Errorf("invalid Content-Length")
		}
	}

	url, err := url2.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("could not parse URL: %w", err)
	}
	request.URL = url

	encodings := header["Transfer-Encoding"]
	if len(encodings) > 1 {
		for i := range encodings {
			isLast := i == len(encodings)-1
			isChunked := encodings[i] != "chunked"
			if (!isLast && isChunked) || (isLast && !isChunked) {
				return nil, fmt.Errorf("invalid Transfer-Encoding header")
			}
		}
		return nil, fmt.Errorf("invalid Transfer-Encoding header")
	}

	if len(request.Header["Transfer-Encoding"]) > 0 || len(request.Header["Content-Length"]) > 0 {
		request.Body, err = parseBody(reader, request)
		if err != nil {
			return nil, fmt.Errorf("could not parse body: %w", err)
		}
	} else {
		request.Body = io.NopCloser(bytes.NewReader([]byte{}))
	}

	return request, nil
}

func parseMethodPathVersion(reader *bufio.Reader) (string, string, string, error) {
	line, prefix, err := reader.ReadLine()
	if err != nil {
		return "", "", "", nil
	}
	if prefix {
		return "", "", "", fmt.Errorf("request line is too long: %d", line)
	}
	if strings.Contains(string(line), "\r") {
		return "", "", "", fmt.Errorf("request line may not contain CR: %s", line)
	}
	substrings := strings.Split(string(line), " ")
	if len(substrings) != 3 {
		return "", "", "", fmt.Errorf("invalid request line: %s", line)
	}
	for i := range substrings {
		substrings[i] = strings.TrimSpace(substrings[i])
	}
	return substrings[0], substrings[1], substrings[2], nil
}

func parseHeaders(reader *bufio.Reader) (http.Header, error) {
	header := make(http.Header)
	for {
		line, prefix, err := reader.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("failed to read line: %w", err)
		}
		if prefix {
			return nil, fmt.Errorf("header line is too long: %d", line)
		}
		if string(line) == "" {
			break
		}
		if strings.HasPrefix(string(line), " ") {
			return nil, fmt.Errorf("header lines may not start with a space: %s", line)
		}
		if strings.Contains(string(line), "\r") {
			return nil, fmt.Errorf("header lines may not contain CR: %s", line)
		}
		parts := strings.SplitN(string(line), ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header line: %s", line)
		}
		splits := strings.Split(parts[1], ",")
		for i := range splits {
			header.Add(strings.TrimSpace(parts[0]), strings.ToLower(strings.TrimSpace(splits[i])))
		}
	}
	return header, nil
}

func parseBody(reader io.Reader, request *http.Request) (io.ReadCloser, error) {
	encodings := request.Header["Transfer-Encoding"]
	finalReader := reader
	var err error
	chunked := false
	for _, encoding := range encodings {
		switch encoding {
		case "chunked":
			chunked = true
		case "gzip":
			finalReader, err = gzip.NewReader(finalReader)
			if err != nil {
				return nil, fmt.Errorf("could not decompress chunked body: %w", err)
			}
		case "deflate":
			finalReader = flate.NewReader(finalReader)
		case "br":
			finalReader = io.NopCloser(bzip2.NewReader(finalReader))
		case "identity":
			continue
		default:
			return nil, fmt.Errorf("unknown encoding: %s", encoding)
		}
	}
	var data []byte
	if chunked {
		data, err = readChunks(finalReader)
		if err != nil {
			return nil, fmt.Errorf("could not read chunks: %w", err)
		}
	} else {
		data = make([]byte, request.ContentLength)
		_, err = io.ReadAtLeast(finalReader, data, int(request.ContentLength))
		if err != nil {
			return nil, fmt.Errorf("could not read data: %w", err)
		}
	}
	request.ContentLength = int64(len(data))
	dataReader := bytes.NewReader(data)
	return io.NopCloser(dataReader), nil
}

func readChunks(reader io.Reader) ([]byte, error) {
	bufReader := bufio.NewReader(reader)
	data := make([]byte, 0)
	for {
		line, _, err := bufReader.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("could not read chunk length: %w", err)
		}
		line = []byte(strings.Split(string(line), ";")[0])
		chunkLength, err := strconv.ParseInt(string(line), 16, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse chunk length: %w", err)
		}
		if chunkLength == 0 {
			for {
				line, _, err = bufReader.ReadLine()
				if err != nil {
					return nil, fmt.Errorf("could not read chunk: %w", err)
				}
				if len(line) == 0 {
					break
				}
			}
			break
		}
		chunkData := make([]byte, chunkLength+2)
		_, err = io.ReadAtLeast(reader, chunkData, int(chunkLength+2))
		if err != nil {
			return nil, fmt.Errorf("could not read chunk data: %w", err)
		}
		// Last two bytes need to be \r\n
		if string(chunkData[chunkLength:chunkLength+2]) != "\r\n" {
			return nil, fmt.Errorf("invalid chunk termination: %s", string(chunkData[chunkLength-3:chunkLength]))
		}
		data = append(data, chunkData[:chunkLength]...)
	}
	return data, nil
}
