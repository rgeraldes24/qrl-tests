// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package api

import (
	"context"
	"encoding/json"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func (suite *liveSuite) assertGraphQLQueries(ctx context.Context) {
	ginkgo.GinkgoHelper()

	fixture := suite.fixture
	data := suite.queryGraphQL(ctx, apiGraphQLQuery, map[string]any{
		"block":   hexutil.EncodeBig(fixture.receipt.BlockNumber),
		"hash":    fixture.block.Hash().Hex(),
		"txHash":  fixture.tx.Hash().Hex(),
		"address": fixture.address.Hex(),
		"sender":  suite.from.Hex(),
		"slot":    (common.Hash{}).Hex(),
		"topic":   fixture.topic.Hex(),
		"index":   hexutil.EncodeUint64(uint64(fixture.receipt.TransactionIndex)),
	})
	var response graphQLQueryResponse
	gomega.Expect(json.Unmarshal(data, &response)).To(gomega.Succeed())

	expected := suite.graphQLQueryExpected(ctx)
	assertGraphQLBlock(response.Block, expected, fixture)
	assertGraphQLSelections(response, expected, suite)
	assertGraphQLWithdrawals(response.Block, fixture)
	assertGraphQLFixtureResults(response, suite)
}
