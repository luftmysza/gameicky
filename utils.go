package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
)

type httpCustom struct {
	client *http.Client
}

func (c *httpCustom) get(url string) ([]byte, error) {
	res, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP :: error executing the HTTP request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("HTTP :: error executing the HTTP request: %v", err)
		}
	}()

	return io.ReadAll(res.Body)
}

func (c *httpCustom) post(url string, contentType string, body io.Reader) ([]byte, error) {
	res, err := c.client.Post(url, contentType, body)
	if err != nil {
		return nil, fmt.Errorf("HTTP :: error executing the HTTP request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("HTTP :: error executing the HTTP request: %v", err)
		}
	}()

	return io.ReadAll(res.Body)
}

func writeJsonToFile(jsonObject *any) error {
	file, err := os.OpenFile("games.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	json_encoder := json.NewEncoder(file)
	json_encoder.SetIndent("", "  ")
	json_encoder.Encode(&jsonObject)

	return nil
}

func getFunctionName() string {
	pc, _, _, _ := runtime.Caller(2)
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	return strings.TrimPrefix(fn.Name(), "main.")
}

func sdebugf(format string, a ...any) string {
	msg := fmt.Sprintf("[D] %s: %s\n", getFunctionName(), fmt.Sprintf(format, a...))
	// fmt.Print(msg)
	return msg
}

func sinfof(format string, a ...any) string {
	msg := fmt.Sprintf("[I] %s: %s\n", getFunctionName(), fmt.Sprintf(format, a...))
	// fmt.Print(msg)
	return msg
}

func swarnf(format string, a ...any) string {
	msg := fmt.Sprintf("[W] %s: %s\n", getFunctionName(), fmt.Sprintf(format, a...))
	// fmt.Print(msg)
	return msg
}

func serrorf(format string, a ...any) string {
	msg := fmt.Sprintf("[E] %s: %s\n", getFunctionName(), fmt.Sprintf(format, a...))
	// fmt.Print(msg)
	return msg
}

func scriticalf(format string, a ...any) string {
	msg := fmt.Sprintf("[C] %s: %s\n", getFunctionName(), fmt.Sprintf(format, a...))
	// fmt.Print(msg)
	return msg
}

func panicf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	msg = fmt.Sprintf("[P] PANIC %s: %s\n", getFunctionName(), msg)
	panic(msg)
}
