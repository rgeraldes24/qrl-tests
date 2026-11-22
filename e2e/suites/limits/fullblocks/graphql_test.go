//go:build e2e

package fullblocks

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cyyber/qrl-tests/e2e/internal/testsuite"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	// The blocks one GraphQL query asks for, with every transaction's bytes.
	queriedBlocks = 8
	// The largest reply the suite reads into memory.
	largestReply = 512 << 20
)

func (suite *fullBlocksSuite) answersGraphQLQuery(ctx ginkgo.SpecContext) {
	gomega.Expect(suite.blocks).NotTo(gomega.BeEmpty(), "there are no full blocks")
	query := fmt.Sprintf(`{"query":"{ blocks(from: %d, to: %d) { number transactions { raw } } }"}`,
		suite.blocks[0].Number, suite.blocks[queriedBlocks-1].Number)
	request := testsuite.MustSucceed(http.NewRequestWithContext(ctx, http.MethodPost,
		suite.source.Participant.Execution.GraphQLURL, bytes.NewReader([]byte(query))))
	request.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := http.DefaultClient.Do(request)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	defer response.Body.Close()
	size, err := io.Copy(io.Discard, io.LimitReader(response.Body, largestReply))
	ginkgo.AddReportEntry("GraphQL query", fmt.Sprintf("%d full blocks: status %d, %d MB in %s, read error: %v",
		queriedBlocks, response.StatusCode, size>>20, time.Since(started).Round(time.Millisecond), err))
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "the reply was cut off")
	gomega.Expect(response.StatusCode).To(gomega.Equal(http.StatusOK))
}
