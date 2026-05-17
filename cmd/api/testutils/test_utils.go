package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetTargetResponse() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

func GetTestContext() *gin.Context {
	gin.SetMode(gin.ReleaseMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

func ConvertJSONFileToString(mockName string, testDataPath string) string {
	extension := filepath.Ext(mockName)
	name := mockName[0 : len(mockName)-len(extension)]

	if strings.Contains(name, ".") {
		return ""
	}

	absPath, err := filepath.Abs(fmt.Sprintf(testDataPath, mockName))
	if err != nil {
		return ""
	}

	fileData, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}

	return string(fileData)
}

func GetJSONMockFromFile(path string, testDataPath string, domain interface{}) interface{} {
	stringFile := strings.NewReader(ConvertJSONFileToString(path, testDataPath))
	buf := new(bytes.Buffer)

	if _, err := buf.ReadFrom(stringFile); err != nil {
		fmt.Printf("read error corrupted data")
	}

	b := buf.Bytes()
	if err := json.Unmarshal(b, domain); err != nil {
		fmt.Printf("unmarshal json error")
	}
	return domain
}

func GetMockedContext(method string, url string, requestBody io.Reader, response *httptest.ResponseRecorder) *gin.Context {
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(method, url, requestBody)
	return c
}

func GetTestGinContext() *gin.Context {
	return &gin.Context{Request: &http.Request{}}
}

func AddParameterToContext(ctx *gin.Context, key string, value string) *gin.Context {
	ctx.Params = append(ctx.Params, gin.Param{Key: key, Value: value})
	return ctx
}
