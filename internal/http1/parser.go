package http1

import (
	"bufio"
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
	request.Method = method
	request.Proto = version
	versionNums := strings.Split(strings.Split(version, "/")[1], ".")
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

	if len(header["Host"]) == 0 {
		return nil, fmt.Errorf("no Host header")
	}
	request.Host = header["Host"][0]

	if len(header["Content-Length"]) == 0 {
		request.ContentLength = 0
	} else {
		request.ContentLength, err = strconv.ParseInt(header["Content-Length"][0], 10, 64)
	}

	url, err := url2.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("could not parse URL: %w", err)
	}
	request.URL = url

	limitedReader := io.LimitReader(reader, request.ContentLength)
	readCloser := io.NopCloser(limitedReader)
	request.Body = readCloser

	return request, nil
}

func parseMethodPathVersion(reader *bufio.Reader) (string, string, string, error) {
	line, prefix, err := reader.ReadLine()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read line: %w", err)
	}
	if prefix {
		return "", "", "", fmt.Errorf("line is too long: %d", line)
	}
	substrings := strings.Split(string(line), " ")
	if len(substrings) != 3 {
		return "", "", "", fmt.Errorf("invalid method path line: %s", line)
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
		parts := strings.SplitN(string(line), ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header line: %s", line)
		}
		header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}
	return header, nil
}
