package metrics

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

const (
	_baseMetricName = "base_repository_name"
)

func IncrementCounter(ctx *gin.Context, metricName string, tags map[string]string) {
	metric := _baseMetricName + "." + metricName
	tagLog := ""

	for key, element := range tags {
		tagLog = tagLog + " || " + key + ":" + element
	}

	fmt.Printf("[METRIC] %s%s\n", metric, tagLog)
}

func SendMetrics(ctx *gin.Context, metricName, metricMethod, metricStatus, step string, tags map[string]string, statusCode int) {
	tags["status"] = metricStatus
	tags["step"] = step
	tags["status_code"] = fmt.Sprintf("%d", statusCode)

	IncrementCounter(ctx, fmt.Sprintf("%s.%s", metricName, metricMethod), tags)
}
