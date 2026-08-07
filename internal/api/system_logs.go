package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

const maximumLogSnapshotBytes int64 = 128 << 10

var (
	errInvalidSystemLogSource = errors.New("invalid system log source")
	errSystemLogDisabled      = errors.New("system log file output is disabled")
)

func (a *APIServer) systemLogs(c *gin.Context) {
	data, err := a.readSystemLogs(c.Query("source"), maximumLogSnapshotBytes)
	if err != nil {
		status := http.StatusNotFound
		code := "system_logs_unavailable"
		if errors.Is(err, errInvalidSystemLogSource) {
			status = http.StatusBadRequest
			code = "invalid_log_source"
		}
		writeError(c.Writer, status, code, err.Error())
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

func (a *APIServer) streamSystemLogs(c *gin.Context) {
	source := c.Query("source")
	if _, err := a.systemLogPath(source); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_log_source", err.Error())
		return
	}
	streamLogSnapshots(c, func() ([]byte, error) {
		return a.readSystemLogs(source, maximumLogSnapshotBytes)
	})
}

func (a *APIServer) readSystemLogs(source string, maximum int64) ([]byte, error) {
	runtime := a.runtimeSnapshot()
	if source == "" || source == "business" {
		if !runtime.Logging.File.Enabled {
			return nil, fmt.Errorf("%w for main application logs", errSystemLogDisabled)
		}
	} else if source == "access" {
		if !runtime.Logging.File.Enabled || !runtime.Logging.File.Access.Enabled {
			return nil, fmt.Errorf("%w for HTTP access logs", errSystemLogDisabled)
		}
	}
	path, err := a.systemLogPath(source)
	if err != nil {
		return nil, err
	}
	return readLogTail(path, maximum)
}

func (a *APIServer) systemLogPath(source string) (string, error) {
	filename := a.config.Logging.File.Filename
	switch source {
	case "", "business":
	case "access":
		filename = a.config.Logging.File.Access.Filename
	default:
		return "", fmt.Errorf("%w: %s", errInvalidSystemLogSource, source)
	}
	return filepath.Join(a.config.Logging.File.Directory, filename), nil
}

func readLogTail(path string, maximum int64) ([]byte, error) {
	if maximum <= 0 || maximum > 1<<20 {
		maximum = maximumLogSnapshotBytes
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	offset := info.Size() - maximum
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(file, maximum))
}

func streamLogSnapshots(c *gin.Context, snapshot func() ([]byte, error)) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	first := true
	hasSnapshot := false
	lastSnapshot := ""
	lastError := ""
	lastHeartbeat := time.Now()
	c.Stream(func(_ io.Writer) bool {
		if first {
			first = false
		} else {
			select {
			case <-c.Request.Context().Done():
				return false
			case <-ticker.C:
			}
		}
		if time.Since(lastHeartbeat) >= 15*time.Second {
			c.SSEvent("heartbeat", map[string]int64{"timestamp": time.Now().Unix()})
			lastHeartbeat = time.Now()
		}
		data, err := snapshot()
		if err != nil {
			if message := err.Error(); message != lastError {
				c.SSEvent("log-error", map[string]string{"message": message})
				lastError = message
			}
			return true
		}
		lastError = ""
		value := string(data)
		if !hasSnapshot || value != lastSnapshot {
			c.SSEvent("snapshot", value)
			lastSnapshot = value
			hasSnapshot = true
		}
		return true
	})
}
