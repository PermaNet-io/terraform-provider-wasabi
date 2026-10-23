package aws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func hasQueryKey(r *http.Request, key string) bool {
	_, ok := r.URL.Query()[key]
	return ok
}

func TestS3BucketRegion(t *testing.T) {
	testCases := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "location constraint",
			body:     `<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-central-1</LocationConstraint>`,
			expected: "us-central-1",
		},
		{
			name:     "null location constraint",
			body:     `<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`,
			expected: "us-east-1",
		},
		{
			name:     "legacy EU constraint",
			body:     `<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">EU</LocationConstraint>`,
			expected: "eu-west-1",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var authorization string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authorization = r.Header.Get("Authorization")
				if r.Method != http.MethodGet || !hasQueryKey(r, "location") {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			sess := session.Must(session.NewSession(&aws.Config{
				Credentials:      credentials.NewStaticCredentials("AKIATEST", "secret", ""),
				Endpoint:         aws.String(server.URL),
				Region:           aws.String("us-east-1"),
				S3ForcePathStyle: aws.Bool(true),
				DisableSSL:       aws.Bool(true),
			}))

			region, err := s3BucketRegion(s3.New(sess), "example")
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if region != tc.expected {
				t.Errorf("expected region %q, got %q", tc.expected, region)
			}
			if authorization == "" {
				t.Error("expected a signed request")
			}
		})
	}
}

func TestS3BucketRegion_error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	sess := session.Must(session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials("AKIATEST", "secret", ""),
		Endpoint:         aws.String(server.URL),
		Region:           aws.String("us-east-1"),
		S3ForcePathStyle: aws.Bool(true),
		DisableSSL:       aws.Bool(true),
		MaxRetries:       aws.Int(0),
	}))

	if _, err := s3BucketRegion(s3.New(sess), "example"); err == nil {
		t.Fatal("expected an error for a 403 response")
	}
}
