// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package api

import (
	"context"
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "API live E2E suite")
}

var suite *liveSuite

var _ = ginkgo.BeforeSuite(func(ctx ginkgo.SpecContext) {
	suite = setupLiveSuite(ctx)
})

type liveScenario struct {
	id        scenarioID
	assertion func(*liveSuite, context.Context)
}

var liveScenarios = []liveScenario{
	{scenarioNodeMetadata, (*liveSuite).assertNodeMetadata},
	{scenarioChainState, (*liveSuite).assertChainState},
	{scenarioTransactions, (*liveSuite).assertTransactions},
	{scenarioTxPool, (*liveSuite).assertTxPool},
	{scenarioRuntimeDiagnostics, (*liveSuite).assertRuntimeDiagnostics},
	{scenarioHistoricalLogs, (*liveSuite).assertHistoricalLogs},
	{scenarioBlockFilter, (*liveSuite).assertBlockFilter},
	{scenarioPendingFilter, (*liveSuite).assertPendingFilter},
	{scenarioSubscriptionEvents, (*liveSuite).assertSubscriptionEvents},
	{scenarioSubscriptionRegistration, (*liveSuite).assertSubscriptionRegistration},
	{scenarioRawDebug, (*liveSuite).assertRawDebug},
	{scenarioDebugState, (*liveSuite).assertDebugState},
	{scenarioDebugTracing, (*liveSuite).assertDebugTracing},
	{scenarioDebugErrorPaths, (*liveSuite).assertDebugErrorPaths},
	{scenarioGraphQLSchema, (*liveSuite).assertGraphQLSchema},
	{scenarioGraphQLQueries, (*liveSuite).assertGraphQLQueries},
	{scenarioGraphQLMutation, (*liveSuite).assertGraphQLMutation},
	{scenarioGraphQLPending, (*liveSuite).assertGraphQLPending},
}

func liveIt(scenario scenarioID, assertion func(*liveSuite, context.Context)) {
	ginkgo.It(scenarioDescriptions[scenario], func(ctx ginkgo.SpecContext) {
		assertion(suite, ctx)
	}, ginkgo.Label(string(scenario)))
}

var _ = ginkgo.Describe(
	"APIs against a live qrl-package network",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "api", "mutates-chain"),
	func() {
		for _, scenario := range liveScenarios {
			liveIt(scenario.id, scenario.assertion)
		}
	},
)
