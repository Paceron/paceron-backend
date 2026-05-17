package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type RestClientConfig struct {
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

func GetScope() string {
	env := GetEnvironment()
	if strings.Contains(strings.ToLower(env), "prod") {
		return "prod"
	}
	if strings.Contains(strings.ToLower(env), "stage") {
		return "stage"
	}
	if strings.Contains(strings.ToLower(env), "test") {
		return "test"
	}
	return "local"
}

func LoadRestClientConfig() RestClientConfig {
	scope := GetScope()
	props := loadPropertiesFile(scope)

	baseURL := props["restclient.base.url"]
	if baseURL == "" {
		baseURL = "https://api.open-meteo.com"
	}

	timeoutSec := parseInt(props["restclient.timeout.seconds"], 30)
	timeout := time.Duration(timeoutSec) * time.Second

	maxRetries := parseInt(props["restclient.max.retries"], 3)

	retryDelayMs := parseInt(props["restclient.retry.delay.ms"], 1000)
	retryDelay := time.Duration(retryDelayMs) * time.Millisecond

	return RestClientConfig{
		BaseURL:    baseURL,
		Timeout:    timeout,
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
	}
}

func loadPropertiesFile(scope string) map[string]string {
	props := make(map[string]string)

	fileName := fmt.Sprintf("application-%s.properties", scope)
	filePath := filepath.Join("cmd", "api", "config", "properties", fileName)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return props
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		props[key] = value
	}

	return props
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}
