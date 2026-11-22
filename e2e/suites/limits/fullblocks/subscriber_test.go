//go:build e2e

package fullblocks

import (
	"errors"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

// subscribeAndStall subscribes to every pending transaction in full, then
// reads nothing until its spec.
func (suite *fullBlocksSuite) subscribeAndStall(ctx ginkgo.SpecContext) {
	var err error
	suite.stalled, _, err = websocket.DefaultDialer.DialContext(ctx, suite.source.Participant.Execution.WebSocketURL, nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	ginkgo.DeferCleanup(suite.stalled.Close)
	subscribe := `{"jsonrpc":"2.0","id":1,"method":"qrl_subscribe","params":["newPendingTransactions",true]}`
	gomega.Expect(suite.stalled.WriteMessage(websocket.TextMessage, []byte(subscribe))).To(gomega.Succeed())
}

func (suite *fullBlocksSuite) handlesStalledSubscriber(ginkgo.SpecContext) {
	gomega.Expect(suite.blocks).NotTo(gomega.BeEmpty(), "there are no full blocks")
	stalled := suite.stalled
	var sent int
	for _, block := range suite.blocks {
		sent += block.Transactions
	}
	// What the node wrote before it gave up is still on the connection.
	var notifications int
	var ended error
	for ended == nil {
		_ = stalled.SetReadDeadline(time.Now().Add(20 * time.Second))
		if _, _, ended = stalled.ReadMessage(); ended == nil {
			notifications++
		}
	}
	ginkgo.AddReportEntry("stalled subscriber", fmt.Sprintf("%d notifications of at least %d transactions, then: %v", notifications, sent, ended))
	gomega.Expect(notifications).To(gomega.BeNumerically("<", sent), "the subscriber did not stall")
	var timeout interface{ Timeout() bool }
	if !errors.As(ended, &timeout) || !timeout.Timeout() {
		return // The node closed the connection.
	}

	// The node kept the connection, so a call on it has to be answered.
	call := `{"jsonrpc":"2.0","id":2,"method":"qrl_chainId","params":[]}`
	gomega.Expect(stalled.WriteMessage(websocket.TextMessage, []byte(call))).To(gomega.Succeed())
	_ = stalled.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, _, err := stalled.ReadMessage()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "the node keeps the connection but no longer writes to it")
}
