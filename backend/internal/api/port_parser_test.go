package api

import "testing"

func TestParseDockerfilePort(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name: "simple EXPOSE",
			content: `FROM alpine:3.20
WORKDIR /app
EXPOSE 8080
CMD ["app"]`,
			expected: 8080,
		},
		{
			name: "EXPOSE with protocol",
			content: `FROM alpine:3.20
EXPOSE 3000/tcp
CMD ["app"]`,
			expected: 3000,
		},
		{
			name: "multiple EXPOSE (takes first)",
			content: `FROM alpine:3.20
EXPOSE 8080
EXPOSE 9090
CMD ["app"]`,
			expected: 8080,
		},
		{
			name:     "no EXPOSE",
			content:  `FROM alpine:3.20\nCMD ["app"]`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDockerfilePort(tt.content)
			if result != tt.expected {
				t.Errorf("parseDockerfilePort() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestParseComposePort(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		serviceName       string
		expectedHost      int
		expectedContainer int
	}{
		{
			name: "simple port mapping",
			content: `version: "3.8"
services:
  web:
    image: nginx
    ports:
      - "8080:80"`,
			serviceName:       "web",
			expectedHost:      8080,
			expectedContainer: 80,
		},
		{
			name: "port mapping with quotes",
			content: `version: "3.8"
services:
  app:
    image: node
    ports:
      - "3000:3000"`,
			serviceName:       "app",
			expectedHost:      3000,
			expectedContainer: 3000,
		},
		{
			name: "multiple ports (takes first)",
			content: `version: "3.8"
services:
  web:
    image: nginx
    ports:
      - "8080:80"
      - "8443:443"`,
			serviceName:       "web",
			expectedHost:      8080,
			expectedContainer: 80,
		},
		{
			name: "no ports",
			content: `version: "3.8"
services:
  web:
    image: nginx`,
			serviceName:       "web",
			expectedHost:      0,
			expectedContainer: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, container := parseComposePort(tt.content, tt.serviceName)
			if host != tt.expectedHost || container != tt.expectedContainer {
				t.Errorf("parseComposePort() = (%d, %d), want (%d, %d)",
					host, container, tt.expectedHost, tt.expectedContainer)
			}
		})
	}
}
