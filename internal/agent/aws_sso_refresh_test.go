package agent

import "testing"

func TestExtractAWSSSOURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "standard aws sso login output",
			output: `If the browser does not open or you wish to use a different device to authorize this request, open the following URL:
https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH`,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extractAWSSSOURL(tt.output); got != tt.want {
				t.Errorf("extractAWSSSOURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
