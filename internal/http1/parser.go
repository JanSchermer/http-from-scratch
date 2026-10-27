package http1

import (
	"bufio"
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

	if len(header["Transfer-Encoding"]) != 0 && len(header["Content-Length"]) != 0 {
		return nil, fmt.Errorf("cannot use both Transfer-Encoding and Content-Length")
	}
	encodings := header["Transfer-Encoding"]
	if len(encodings) > 1 {
		for i := range encodings {
			if i == len(encodings)-1 {
				if encodings[i] != "chunked" {
					return nil, fmt.Errorf("invalid Transfer-Encoding header")
				}
			} else {
				if encodings[i] == "chunked" {
					return nil, fmt.Errorf("invalid Transfer-Encoding header")
				}
			}
		}
		return nil, fmt.Errorf("invalid Transfer-Encoding header")
	}

	var returnReader io.ReadCloser
	if len(encodings) == 0 {
		if len(header["Content-Length"]) == 0 {
			returnReader = io.NopCloser(reader)
		} else {
			limitedReader := io.LimitReader(reader, request.ContentLength)
			returnReader = io.NopCloser(limitedReader)
		}
	} else {
		intermediateReader := io.NopCloser(reader)
		for _, encoding := range encodings {
			switch encoding {
			case "chunked":
				panic("unimplemented")
			case "gzip":
				intermediateReader, err = gzip.NewReader(intermediateReader)
				if err != nil {
					return nil, fmt.Errorf("could not decompress chunked body: %w", err)
				}
			case "deflate":
				intermediateReader = flate.NewReader(intermediateReader)
			case "br":
				intermediateReader = io.NopCloser(bzip2.NewReader(intermediateReader))
			case "identity":
				continue
			default:
				return nil, fmt.Errorf("unknown encoding: %s", encoding)

			}
		}
		returnReader = intermediateReader
	}

	request.Body = returnReader

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
			header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(splits[i]))
		}
	}
	return header, nil
}
