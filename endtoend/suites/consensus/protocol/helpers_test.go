//go:build e2e

package protocol_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func (suite *protocolSuite) previousEpoch(ctx context.Context) (uint64, uint64) {
	ginkgo.GinkgoHelper()

	head, err := suite.beacons[0].HeadSlot(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	epoch := head / suite.slotsPerEpoch
	gomega.Expect(epoch).To(gomega.BeNumerically(">", 0))
	start := (epoch - 1) * suite.slotsPerEpoch
	return start, start + suite.slotsPerEpoch
}

func readMetrics(ctx context.Context, endpoint string) map[string]*dto.MetricFamily {
	ginkgo.GinkgoHelper()
	gomega.Expect(endpoint).NotTo(gomega.BeEmpty())
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/metrics", nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	response, err := http.DefaultClient.Do(request)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	defer response.Body.Close()
	gomega.Expect(response.StatusCode).To(gomega.Equal(http.StatusOK))
	parser := expfmt.TextParser{}
	families, err := parser.TextToMetricFamilies(io.LimitReader(response.Body, 32<<20))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return families
}

func singleMetric(families map[string]*dto.MetricFamily, name string) float64 {
	ginkgo.GinkgoHelper()
	values := metricValues(families, name)
	gomega.Expect(values).To(gomega.HaveLen(1), fmt.Sprintf("metric %s", name))
	return values[0]
}

func metricCount(families map[string]*dto.MetricFamily, name string) int {
	return len(metricValues(families, name))
}

func metricSum(families map[string]*dto.MetricFamily, name string) float64 {
	values := metricValues(families, name)
	gomega.Expect(values).NotTo(gomega.BeEmpty(), fmt.Sprintf("metric %s", name))
	var total float64
	for _, value := range values {
		total += value
	}
	return total
}

func metricValues(families map[string]*dto.MetricFamily, name string) []float64 {
	family := families[name]
	if family == nil {
		return nil
	}
	values := make([]float64, 0, len(family.Metric))
	for _, metric := range family.Metric {
		switch family.GetType() {
		case dto.MetricType_COUNTER:
			values = append(values, metric.GetCounter().GetValue())
		case dto.MetricType_GAUGE:
			values = append(values, metric.GetGauge().GetValue())
		case dto.MetricType_UNTYPED:
			values = append(values, metric.GetUntyped().GetValue())
		}
	}
	return values
}
