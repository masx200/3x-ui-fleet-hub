package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/json_util"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/gorm"
)

// OutboundService provides business logic for managing Xray outbound configurations.
// It handles outbound traffic monitoring and statistics.
type OutboundService struct{}

// testSemaphore limits concurrent outbound tests to prevent resource exhaustion.
var testSemaphore sync.Mutex

// BatchTestTask represents an async batch test task
type BatchTestTask struct {
	ID              string                   `json:"id"`
	Status          string                   `json:"status"` // running, completed, cancelled, failed
	Progress        int                      `json:"progress"`
	Total           int                      `json:"total"`
	Current         int                      `json:"current"`
	Results         []*TestAllOutboundResult `json:"results"`
	StartTime       time.Time                `json:"startTime"`
	EndTime         *time.Time               `json:"endTime,omitempty"`
	TotalTime       int64                    `json:"totalTime"`
	Cancel          chan struct{}            `json:"-"`
	SuccessCount    int                      `json:"successCount"`
	FailedCount     int                      `json:"failedCount"`
}

// BatchTestTaskManager manages async batch test tasks
type BatchTestTaskManager struct {
	tasks map[string]*BatchTestTask
	mu    sync.RWMutex
}

var batchTestTaskManager = &BatchTestTaskManager{
	tasks: make(map[string]*BatchTestTask),
}

// CreateTask creates a new batch test task
func (m *BatchTestTaskManager) CreateTask(total int) *BatchTestTask {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &BatchTestTask{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Status:    "running",
		Progress:  0,
		Total:     total,
		Current:   0,
		Results:   make([]*TestAllOutboundResult, 0, total),
		StartTime: time.Now(),
		Cancel:    make(chan struct{}),
	}

	m.tasks[task.ID] = task
	return task
}

// GetTask retrieves a task by ID
func (m *BatchTestTaskManager) GetTask(id string) (*BatchTestTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	return task, exists
}

// UpdateTask updates a task's progress
func (m *BatchTestTaskManager) UpdateTask(id string, current int, result *TestAllOutboundResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.tasks[id]; exists {
		task.Current = current
		task.Progress = int(float64(current) / float64(task.Total) * 100)
		if result != nil {
			task.Results = append(task.Results, result)
			if result.Success {
				task.SuccessCount++
			} else if result.Error == "" || !contains(result.Error, "Skipped") {
				task.FailedCount++
			}
		}
	}
}

// CompleteTask marks a task as completed
func (m *BatchTestTaskManager) CompleteTask(id string, status string, totalTime int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.tasks[id]; exists {
		task.Status = status
		task.Progress = 100
		task.TotalTime = totalTime
		endTime := time.Now()
		task.EndTime = &endTime
	}
}

// CancelTask cancels a running task
func (m *BatchTestTaskManager) CancelTask(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.tasks[id]; exists && task.Status == "running" {
		close(task.Cancel)
		task.Status = "cancelled"
		endTime := time.Now()
		task.EndTime = &endTime
		return true
	}
	return false
}

// CleanOldTasks removes tasks older than 1 hour
func (m *BatchTestTaskManager) CleanOldTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-time.Hour)
	for id, task := range m.tasks {
		if !task.EndTime.IsZero() && task.EndTime.Before(cutoff) {
			delete(m.tasks, id)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (s *OutboundService) AddTraffic(traffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (error, bool) {
	var err error
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = s.addOutboundTraffic(tx, traffics)
	if err != nil {
		return err, false
	}

	return nil, false
}

func (s *OutboundService) addOutboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsOutbound {

			var outbound model.OutboundTraffics

			err = tx.Model(&model.OutboundTraffics{}).Where("tag = ?", traffic.Tag).
				FirstOrCreate(&outbound).Error
			if err != nil {
				return err
			}

			outbound.Tag = traffic.Tag
			outbound.Up = outbound.Up + traffic.Up
			outbound.Down = outbound.Down + traffic.Down
			outbound.Total = outbound.Up + outbound.Down

			err = tx.Save(&outbound).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *OutboundService) GetOutboundsTraffic() ([]*model.OutboundTraffics, error) {
	db := database.GetDB()
	var traffics []*model.OutboundTraffics

	err := db.Model(model.OutboundTraffics{}).Find(&traffics).Error
	if err != nil {
		logger.Warning("Error retrieving OutboundTraffics: ", err)
		return nil, err
	}

	return traffics, nil
}

func (s *OutboundService) ResetOutboundTraffic(tag string) error {
	db := database.GetDB()

	whereText := "tag "
	if tag == "-alltags-" {
		whereText += " <> ?"
	} else {
		whereText += " = ?"
	}

	result := db.Model(model.OutboundTraffics{}).
		Where(whereText, tag).
		Updates(map[string]any{"up": 0, "down": 0, "total": 0})

	err := result.Error
	if err != nil {
		return err
	}

	return nil
}

// TestOutboundResult represents the result of testing an outbound
type TestOutboundResult struct {
	Success    bool   `json:"success"`
	Delay      int64  `json:"delay"` // Delay in milliseconds
	Error      string `json:"error,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
}

// TestAllOutboundResult represents the result of testing a single outbound in batch mode
type TestAllOutboundResult struct {
	Index      int    `json:"index"`
	Tag        string `json:"tag"`
	Success    bool   `json:"success"`
	Delay      int64  `json:"delay"`
	Error      string `json:"error,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
}

// TestAllOutboundsResult represents the result of testing all outbounds
type TestAllOutboundsResult struct {
	Total     int                      `json:"total"`
	Tested    int                      `json:"tested"`
	Results   []*TestAllOutboundResult `json:"results"`
	TotalTime int64                    `json:"totalTime"`
}

// TestOutbound tests an outbound by creating a temporary xray instance and measuring response time.
// allOutboundsJSON must be a JSON array of all outbounds; they are copied into the test config unchanged.
// Only the test inbound and a route rule (to the tested outbound tag) are added.
func (s *OutboundService) TestOutbound(outboundJSON string, testURL string, allOutboundsJSON string) (*TestOutboundResult, error) {
	if testURL == "" {
		testURL = "https://www.google.com/generate_204"
	}

	// Limit to one concurrent test at a time
	if !testSemaphore.TryLock() {
		return &TestOutboundResult{
			Success: false,
			Error:   "Another outbound test is already running, please wait",
		}, nil
	}
	defer testSemaphore.Unlock()

	// Parse the outbound being tested to get its tag
	var testOutbound map[string]any
	if err := json.Unmarshal([]byte(outboundJSON), &testOutbound); err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid outbound JSON: %v", err),
		}, nil
	}
	outboundTag, _ := testOutbound["tag"].(string)
	if outboundTag == "" {
		return &TestOutboundResult{
			Success: false,
			Error:   "Outbound has no tag",
		}, nil
	}
	if protocol, _ := testOutbound["protocol"].(string); protocol == "blackhole" || outboundTag == "blocked" {
		return &TestOutboundResult{
			Success: false,
			Error:   "Blocked/blackhole outbound cannot be tested",
		}, nil
	}

	// Use all outbounds when provided; otherwise fall back to single outbound
	var allOutbounds []any
	if allOutboundsJSON != "" {
		if err := json.Unmarshal([]byte(allOutboundsJSON), &allOutbounds); err != nil {
			return &TestOutboundResult{
				Success: false,
				Error:   fmt.Sprintf("Invalid allOutbounds JSON: %v", err),
			}, nil
		}
	}
	if len(allOutbounds) == 0 {
		allOutbounds = []any{testOutbound}
	}

	// Find an available port for test inbound
	testPort, err := findAvailablePort()
	if err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to find available port: %v", err),
		}, nil
	}

	// Copy all outbounds as-is, add only test inbound and route rule
	testConfig := s.createTestConfig(outboundTag, allOutbounds, testPort)

	// Use a temporary config file so the main config.json is never overwritten
	testConfigPath, err := createTestConfigPath()
	if err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to create test config path: %v", err),
		}, nil
	}
	defer os.Remove(testConfigPath) // ensure temp file is removed even if process is not stopped

	// Create temporary xray process with its own config file
	testProcess := xray.NewTestProcess(testConfig, testConfigPath)
	defer func() {
		if testProcess.IsRunning() {
			testProcess.Stop()
		}
	}()

	// Start the test process
	if err := testProcess.Start(); err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to start test xray instance: %v", err),
		}, nil
	}

	// Wait for xray to start listening on the test port
	if err := waitForPort(testPort, 3*time.Second); err != nil {
		if !testProcess.IsRunning() {
			result := testProcess.GetResult()
			return &TestOutboundResult{
				Success: false,
				Error:   fmt.Sprintf("Xray process exited: %s", result),
			}, nil
		}
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Xray failed to start listening: %v", err),
		}, nil
	}

	// Check if process is still running
	if !testProcess.IsRunning() {
		result := testProcess.GetResult()
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Xray process exited: %s", result),
		}, nil
	}

	// Test the connection through proxy
	delay, statusCode, err := s.testConnection(testPort, testURL)
	if err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &TestOutboundResult{
		Success:    true,
		Delay:      delay,
		StatusCode: statusCode,
	}, nil
}

// createTestConfig creates a test config by copying all outbounds unchanged and adding
// only the test inbound (SOCKS) and a route rule that sends traffic to the given outbound tag.
func (s *OutboundService) createTestConfig(outboundTag string, allOutbounds []any, testPort int) *xray.Config {
	// Test inbound (SOCKS proxy) - only addition to inbounds
	testInbound := xray.InboundConfig{
		Tag:      "test-inbound",
		Listen:   json_util.RawMessage(`"127.0.0.1"`),
		Port:     testPort,
		Protocol: "socks",
		Settings: json_util.RawMessage(`{"auth":"noauth","udp":true}`),
	}

	// Outbounds: copy all, but set noKernelTun=true for WireGuard outbounds
	processedOutbounds := make([]any, len(allOutbounds))
	for i, ob := range allOutbounds {
		outbound, ok := ob.(map[string]any)
		if !ok {
			processedOutbounds[i] = ob
			continue
		}
		if protocol, ok := outbound["protocol"].(string); ok && protocol == "wireguard" {
			// Set noKernelTun to true for WireGuard outbounds
			if settings, ok := outbound["settings"].(map[string]any); ok {
				settings["noKernelTun"] = true
			} else {
				// Create settings if it doesn't exist
				outbound["settings"] = map[string]any{
					"noKernelTun": true,
				}
			}
		}
		processedOutbounds[i] = outbound
	}
	outboundsJSON, _ := json.Marshal(processedOutbounds)

	// Create routing rule to route all traffic through test outbound
	routingRules := []map[string]any{
		{
			"type":        "field",
			"outboundTag": outboundTag,
			"network":     "tcp,udp",
		},
	}

	routingJSON, _ := json.Marshal(map[string]any{
		"domainStrategy": "AsIs",
		"rules":          routingRules,
	})

	// Disable logging for test process to avoid creating orphaned log files
	logConfig := map[string]any{
		"loglevel": "warning",
		"access":   "none",
		"error":    "none",
		"dnsLog":   false,
	}
	logJSON, _ := json.Marshal(logConfig)

	// Create minimal config
	cfg := &xray.Config{
		LogConfig: json_util.RawMessage(logJSON),
		InboundConfigs: []xray.InboundConfig{
			testInbound,
		},
		OutboundConfigs: json_util.RawMessage(string(outboundsJSON)),
		RouterConfig:    json_util.RawMessage(string(routingJSON)),
		Policy:          json_util.RawMessage(`{}`),
		Stats:           json_util.RawMessage(`{}`),
	}

	return cfg
}

// testConnection tests the connection through the proxy and measures delay.
// It performs a warmup request first to establish the SOCKS connection and populate DNS caches,
// then measures the second request for a more accurate latency reading.
func (s *OutboundService) testConnection(proxyPort int, testURL string) (int64, int, error) {
	// Create SOCKS5 proxy URL
	proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", proxyPort)

	// Parse proxy URL
	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return 0, 0, common.NewErrorf("Invalid proxy URL: %v", err)
	}

	// Create HTTP client with proxy and keep-alive for connection reuse
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:       1,
			IdleConnTimeout:    10 * time.Second,
			DisableCompression: true,
		},
	}

	// Warmup request: establishes SOCKS/TLS connection, DNS, and TCP to the target.
	// This mirrors real-world usage where connections are reused.
	warmupResp, err := client.Get(testURL)
	if err != nil {
		return 0, 0, common.NewErrorf("Request failed: %v", err)
	}
	io.Copy(io.Discard, warmupResp.Body)
	warmupResp.Body.Close()

	// Measure the actual request on the warm connection
	startTime := time.Now()
	resp, err := client.Get(testURL)
	delay := time.Since(startTime).Milliseconds()

	if err != nil {
		return 0, 0, common.NewErrorf("Request failed: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return delay, resp.StatusCode, nil
}

// waitForPort polls until the given TCP port is accepting connections or the timeout expires.
func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("port %d not ready after %v", port, timeout)
}

// findAvailablePort finds an available port for testing
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// createTestConfigPath returns a unique path for a temporary xray config file in the bin folder.
// The temp file is created and closed so the path is reserved; Start() will overwrite it.
func createTestConfigPath() (string, error) {
	tmpFile, err := os.CreateTemp(config.GetBinFolderPath(), "xray_test_*.json")
	if err != nil {
		return "", err
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// TestAllOutbounds tests multiple outbounds sequentially and returns comprehensive results
func (s *OutboundService) TestAllOutbounds(allOutboundsJSON string) (*TestAllOutboundsResult, error) {
	startTime := time.Now()

	var allOutbounds []map[string]any
	if err := json.Unmarshal([]byte(allOutboundsJSON), &allOutbounds); err != nil {
		return nil, fmt.Errorf("invalid allOutbounds JSON: %v", err)
	}

	total := len(allOutbounds)
	results := make([]*TestAllOutboundResult, 0, total)
	tested := 0

	testURL := "https://www.google.com/generate_204"

	for i, outbound := range allOutbounds {
		tag, _ := outbound["tag"].(string)
		protocol, _ := outbound["protocol"].(string)

		// Skip blackhole and blocked outbounds
		if protocol == "blackhole" || tag == "blocked" {
			results = append(results, &TestAllOutboundResult{
				Index:   i,
				Tag:     tag,
				Success: false,
				Error:   "Skipped: blocked/blackhole outbound",
			})
			tested++
			continue
		}

		// Test single outbound (reuse existing method)
		outboundJSON, _ := json.Marshal(outbound)
		singleResult, err := s.TestOutbound(string(outboundJSON), testURL, allOutboundsJSON)

		tested++

		if err != nil {
			results = append(results, &TestAllOutboundResult{
				Index:   i,
				Tag:     tag,
				Success: false,
				Error:   err.Error(),
			})
		} else if singleResult != nil {
			results = append(results, &TestAllOutboundResult{
				Index:      i,
				Tag:        tag,
				Success:    singleResult.Success,
				Delay:      singleResult.Delay,
				Error:      singleResult.Error,
				StatusCode: singleResult.StatusCode,
			})
		}
	}

	totalTime := time.Since(startTime).Milliseconds()

	return &TestAllOutboundsResult{
		Total:     total,
		Tested:    tested,
		Results:   results,
		TotalTime: totalTime,
	}, nil
}

// StartAsyncTestAllOutbounds starts an async batch test task and returns task ID
func (s *OutboundService) StartAsyncTestAllOutbounds(allOutboundsJSON string) (string, error) {
	var allOutbounds []map[string]any
	if err := json.Unmarshal([]byte(allOutboundsJSON), &allOutbounds); err != nil {
		return "", fmt.Errorf("invalid allOutbounds JSON: %v", err)
	}

	// Create new task
	task := batchTestTaskManager.CreateTask(len(allOutbounds))

	// Start async execution
	go s.executeAsyncTestAllOutbounds(task, allOutbounds, allOutboundsJSON)

	return task.ID, nil
}

// executeAsyncTestAllOutbounds executes the batch test in the background
func (s *OutboundService) executeAsyncTestAllOutbounds(task *BatchTestTask, allOutbounds []map[string]any, allOutboundsJSON string) {
	testURL := "https://www.google.com/generate_204"
	tested := 0

	for i, outbound := range allOutbounds {
		// Check if task was cancelled
		select {
		case <-task.Cancel:
			batchTestTaskManager.CompleteTask(task.ID, "cancelled", time.Since(task.StartTime).Milliseconds())
			return
		default:
		}

		tag, _ := outbound["tag"].(string)
		protocol, _ := outbound["protocol"].(string)

		// Skip blackhole and blocked outbounds
		if protocol == "blackhole" || tag == "blocked" {
			result := &TestAllOutboundResult{
				Index:   i,
				Tag:     tag,
				Success: false,
				Error:   "Skipped: blocked/blackhole outbound",
			}
			batchTestTaskManager.UpdateTask(task.ID, tested+1, result)
			tested++
			continue
		}

		// Test single outbound
		outboundJSON, _ := json.Marshal(outbound)
		singleResult, err := s.TestOutbound(string(outboundJSON), testURL, allOutboundsJSON)

		tested++

		var result *TestAllOutboundResult
		if err != nil {
			result = &TestAllOutboundResult{
				Index:   i,
				Tag:     tag,
				Success: false,
				Error:   err.Error(),
			}
		} else if singleResult != nil {
			result = &TestAllOutboundResult{
				Index:      i,
				Tag:        tag,
				Success:    singleResult.Success,
				Delay:      singleResult.Delay,
				Error:      singleResult.Error,
				StatusCode: singleResult.StatusCode,
			}
		}

		batchTestTaskManager.UpdateTask(task.ID, tested, result)
	}

	totalTime := time.Since(task.StartTime).Milliseconds()
	batchTestTaskManager.CompleteTask(task.ID, "completed", totalTime)
}

// GetBatchTestTask retrieves a batch test task by ID
func (s *OutboundService) GetBatchTestTask(taskID string) (*BatchTestTask, bool) {
	return batchTestTaskManager.GetTask(taskID)
}

// CancelBatchTestTask cancels a running batch test task
func (s *OutboundService) CancelBatchTestTask(taskID string) bool {
	return batchTestTaskManager.CancelTask(taskID)
}
