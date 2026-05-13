package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoopQuotaStore(t *testing.T) {
	s := noopQuotaStore{}

	ok, err := s.CheckUserQuota(context.Background(), "user1", 1000)
	assert.NoError(t, err)
	assert.True(t, ok)

	err = s.IncrUserTokens(context.Background(), "user1", 500)
	assert.NoError(t, err)

	ok, err = s.CheckGlobalQuota(context.Background(), 1000000)
	assert.NoError(t, err)
	assert.True(t, ok)

	err = s.IncrGlobalCost(context.Background(), 500)
	assert.NoError(t, err)
}
