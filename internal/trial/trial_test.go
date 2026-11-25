package trial

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisabledNeverOffersATrial(t *testing.T) {
	t.Parallel()

	offer, err := Disabled{}.Check(context.Background())
	require.ErrorIs(t, err, ErrUnavailable)
	require.Equal(t, Offer{}, offer)
}
