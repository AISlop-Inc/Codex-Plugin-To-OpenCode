package portal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Serve forwards concurrent requests while keeping each stdout response on one line.
func Serve(ctx context.Context, input io.Reader, output, diagnostics io.Writer, client *Client) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var pending sync.WaitGroup
	var outputMu sync.Mutex
	var writeError error
	write := func(response json.RawMessage, diagnostic error) {
		outputMu.Lock()
		defer outputMu.Unlock()
		if writeError != nil {
			return
		}
		if diagnostic != nil {
			_, writeError = fmt.Fprintln(diagnostics, diagnostic)
		} else if response != nil {
			_, writeError = fmt.Fprintln(output, string(response))
		}
		if writeError != nil {
			cancel()
		}
	}

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), maxMessageSize)
	for scanner.Scan() {
		message, err := parseRequest(scanner.Bytes())
		if err != nil {
			write(errorResponse(nil, -32600, "Invalid MCP request."), nil)
			continue
		}
		pending.Add(1)
		go func() {
			defer pending.Done()
			response, err := client.relay(ctx, message)
			if err != nil {
				if message.ID == nil {
					write(nil, err)
				} else {
					write(errorResponse(message.ID, -32000, err.Error()), nil)
				}
				return
			}
			write(response, nil)
		}()
	}
	if scanner.Err() != nil {
		cancel()
	}
	pending.Wait()
	if writeError != nil {
		return errors.New("Could not write MCP output.")
	}
	if scanner.Err() != nil {
		return errors.New("MCP input unreadable or exceeds 32 MiB.")
	}
	return nil
}
