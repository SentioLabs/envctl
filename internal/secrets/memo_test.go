package secrets_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	envctlerrors "github.com/sentiolabs/envctl/internal/errors"
	"github.com/sentiolabs/envctl/internal/mocks"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const memoScope = "1pass|acct|vault"

func TestMemo_GetSecretReachesBackendOnceAcrossClients(t *testing.T) {
	memo := secrets.NewMemo()
	inner := mocks.NewMockClient(t)
	inner.On("GetSecret", mock.Anything, "Shared").Return(map[string]string{"A": "1"}, nil).Once()

	// Two wrapped clients stand in for two applications in one run.
	first := memo.Wrap(inner, memoScope)
	second := memo.Wrap(inner, memoScope)

	got, err := first.GetSecret(context.Background(), "Shared")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"A": "1"}, got)

	got, err = second.GetSecret(context.Background(), "Shared")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"A": "1"}, got)
}

func TestMemo_GetSecretKeyReachesBackendOnce(t *testing.T) {
	memo := secrets.NewMemo()
	inner := mocks.NewMockClient(t)
	inner.On("GetSecretKey", mock.Anything, "Core", "TENANT").Return("t1", nil).Once()
	client := memo.Wrap(inner, memoScope)

	for range 2 {
		got, err := client.GetSecretKey(context.Background(), "Core", "TENANT")
		require.NoError(t, err)
		assert.Equal(t, "t1", got)
	}
}

func TestMemo_WholeSecretNeverAnswersKeyedRead(t *testing.T) {
	memo := secrets.NewMemo()
	inner := mocks.NewMockClient(t)
	inner.On("GetSecret", mock.Anything, "Core").Return(map[string]string{"TENANT": "from-item"}, nil).Once()
	inner.On("GetSecretKey", mock.Anything, "Core", "TENANT").Return("from-key", nil).Once()
	client := memo.Wrap(inner, memoScope)

	_, err := client.GetSecret(context.Background(), "Core")
	require.NoError(t, err)
	got, err := client.GetSecretKey(context.Background(), "Core", "TENANT")
	require.NoError(t, err)
	assert.Equal(t, "from-key", got)
}

func TestMemo_ScopesStayDistinct(t *testing.T) {
	memo := secrets.NewMemo()
	east := mocks.NewMockClient(t)
	east.On("GetSecret", mock.Anything, "app").Return(map[string]string{"R": "east"}, nil).Once()
	west := mocks.NewMockClient(t)
	west.On("GetSecret", mock.Anything, "app").Return(map[string]string{"R": "west"}, nil).Once()

	gotEast, err := memo.Wrap(east, "aws|us-east-1|p").GetSecret(context.Background(), "app")
	require.NoError(t, err)
	gotWest, err := memo.Wrap(west, "aws|us-west-2|p").GetSecret(context.Background(), "app")
	require.NoError(t, err)

	assert.Equal(t, "east", gotEast["R"])
	assert.Equal(t, "west", gotWest["R"])
}

func TestMemo_ErrorsAreNotRemembered(t *testing.T) {
	memo := secrets.NewMemo()
	inner := mocks.NewMockClient(t)
	boom := errors.New("locked")
	inner.On("GetSecret", mock.Anything, "Shared").Return(nil, boom).Once()
	inner.On("GetSecret", mock.Anything, "Shared").Return(map[string]string{"A": "1"}, nil).Once()
	client := memo.Wrap(inner, memoScope)

	_, err := client.GetSecret(context.Background(), "Shared")
	require.ErrorIs(t, err, boom)
	got, err := client.GetSecret(context.Background(), "Shared")
	require.NoError(t, err)
	assert.Equal(t, "1", got["A"])
}

func TestMemo_CallerEditsDoNotLeakIntoMemo(t *testing.T) {
	memo := secrets.NewMemo()
	inner := mocks.NewMockClient(t)
	inner.On("GetSecret", mock.Anything, "Shared").Return(map[string]string{"A": "1"}, nil).Once()
	client := memo.Wrap(inner, memoScope)

	got, err := client.GetSecret(context.Background(), "Shared")
	require.NoError(t, err)
	got["A"] = "edited"

	again, err := client.GetSecret(context.Background(), "Shared")
	require.NoError(t, err)
	assert.Equal(t, "1", again["A"])
}

func TestMemo_NilWrapReturnsClientUnchanged(t *testing.T) {
	var memo *secrets.Memo
	inner := mocks.NewMockClient(t)
	assert.Same(t, secrets.Client(inner), memo.Wrap(inner, memoScope))
}

func TestMemo_RawReadWithoutRawBackendKeepsBuilderError(t *testing.T) {
	inner := mocks.NewMockClient(t)
	inner.On("Name").Return("plain")
	client := secrets.NewMemo().Wrap(inner, memoScope)

	raw, ok := client.(secrets.RawReader)
	require.True(t, ok)
	_, err := raw.GetSecretRaw(context.Background(), "kube")

	var want *envctlerrors.FileSinkKeyRequiredError
	require.ErrorAs(t, err, &want)
	assert.Equal(t, "plain", want.Backend)
}

func TestMemo_RawReadPassesThrough(t *testing.T) {
	inner := mocks.NewMockRawClient(t)
	inner.Raw.On("GetSecretRaw", mock.Anything, "kube").Return([]byte("pem"), nil).Twice()
	client := secrets.NewMemo().Wrap(inner, memoScope)

	for range 2 {
		got, err := client.(secrets.RawReader).GetSecretRaw(context.Background(), "kube")
		require.NoError(t, err)
		assert.Equal(t, []byte("pem"), got)
	}
}

func TestScope_SeparatesBackendSettings(t *testing.T) {
	cfg := &config.Config{
		DefaultBackend: config.Backend1Pass,
		OnePass:        &config.OnePassConfig{Account: "acct", Vault: "vault"},
	}
	assert.Equal(t, "1pass|acct|vault", secrets.Scope(cfg, nil))
	assert.Equal(t, config.BackendAWS, secrets.Scope(nil, nil))
}
