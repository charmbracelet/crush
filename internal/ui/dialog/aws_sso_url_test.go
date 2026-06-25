package dialog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractAWSSSOURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "standard aws sso login output",
			output: `Attempting to automatically open the SSO authorization page in your default browser.
If the browser does not open or you wish to use a different device to authorize this request, open the following URL:

https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH

Then enter the code: ABCD-EFGH`,
			want: "https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH",
		},
		{
			name:   "url only",
			output: "https://device.sso.eu-west-1.amazonaws.com/?user_code=XXXX-YYYY",
			want:   "https://device.sso.eu-west-1.amazonaws.com/?user_code=XXXX-YYYY",
		},
		{
			name:   "no url",
			output: "SSO session expired. Please run aws sso login.",
			want:   "",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
		{
			name: "multiple urls returns first",
			output: `Open https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH
Also see https://example.com/docs`,
			want: "https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractAWSSSOURL(tt.output)
			require.Equal(t, tt.want, got)
		})
	}
}
