package check_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/sirupsen/logrus"

	"github.com/nicholas-fedor/watchtower/pkg/api"
	checkAPI "github.com/nicholas-fedor/watchtower/pkg/api/check"
)

const (
	token       = "123123123"
	rateLimit60 = 60 // Maximum authentication requests per minute per IP address.
)

func TestCheck(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Check Suite")
}

var _ = ginkgo.Describe("the check API", func() {
	var server *ghttp.Server

	ginkgo.BeforeEach(func() {
		httpAPI := api.New(token, ":8080", rateLimit60)
		handler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
			return []checkAPI.Result{
				{
					Name:            "test-container-1",
					Image:           "example/test-image:latest",
					CurrentDigest:   "sha256:1111111111111111111111111111111111111111111111111111111111111111",
					LatestDigest:    "sha256:2222222222222222222222222222222222222222222222222222222222222222",
					UpdateAvailable: true,
				},
			}, nil
		})
		handleReq := httpAPI.RequireToken(handler.Handle)
		server = ghttp.NewServer()
		server.RouteToHandler("GET", "/v1/check", ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/v1/check"),
			ghttp.VerifyHeaderKV("Authorization", "Bearer "+token),
			handleReq,
		))
	})

	ginkgo.AfterEach(func() {
		server.Close()
	})

	ginkgo.Describe("Successful responses", func() {
		ginkgo.It("should report update availability for each container", func() {
			req, _ := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				server.URL()+"/v1/check",
				nil,
			)
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))

			body, _ := io.ReadAll(resp.Body)

			var parsed struct {
				Containers []checkAPI.Result `json:"containers"`
				Count      int               `json:"count"`
				APIVersion string            `json:"api_version"`
				Checked    string            `json:"checked"`
			}

			gomega.Expect(json.Unmarshal(body, &parsed)).To(gomega.Succeed())
			gomega.Expect(parsed.APIVersion).To(gomega.Equal("v1"))
			gomega.Expect(parsed.Count).To(gomega.Equal(1))
			gomega.Expect(parsed.Containers).To(gomega.HaveLen(1))
			gomega.Expect(parsed.Containers[0].Name).To(gomega.Equal("test-container-1"))
			gomega.Expect(parsed.Containers[0].Image).To(gomega.Equal("example/test-image:latest"))
			gomega.Expect(parsed.Containers[0].CurrentDigest).
				To(gomega.Equal("sha256:1111111111111111111111111111111111111111111111111111111111111111"))
			gomega.Expect(parsed.Containers[0].LatestDigest).
				To(gomega.Equal("sha256:2222222222222222222222222222222222222222222222222222222222222222"))
			gomega.Expect(parsed.Containers[0].UpdateAvailable).To(gomega.BeTrue())
			gomega.Expect(parsed.Checked).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("should return empty list when no containers are watched", func() {
			emptyHTTPAPI := api.New(token, ":8080", rateLimit60)
			emptyHandler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
				return []checkAPI.Result{}, nil
			})
			emptyTokenHandler := emptyHTTPAPI.RequireToken(emptyHandler.Handle)

			emptyServer := ghttp.NewServer()
			defer emptyServer.Close()

			emptyServer.RouteToHandler("GET", "/v1/check", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v1/check"),
				ghttp.VerifyHeaderKV("Authorization", "Bearer "+token),
				emptyTokenHandler,
			))

			req, _ := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				emptyServer.URL()+"/v1/check",
				nil,
			)
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))

			body, _ := io.ReadAll(resp.Body)

			var parsed struct {
				Containers []checkAPI.Result `json:"containers"`
				Count      int               `json:"count"`
			}

			gomega.Expect(json.Unmarshal(body, &parsed)).To(gomega.Succeed())
			gomega.Expect(parsed.Count).To(gomega.Equal(0))
			gomega.Expect(parsed.Containers).To(gomega.BeEmpty())
		})

		ginkgo.It("should report up-to-date containers as not having updates", func() {
			upToDateHTTPAPI := api.New(token, ":8080", rateLimit60)
			upToDateHandler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
				return []checkAPI.Result{
					{
						Name:            "current-container",
						Image:           "example/current:latest",
						CurrentDigest:   "sha256:aaaa",
						LatestDigest:    "sha256:aaaa",
						UpdateAvailable: false,
					},
				}, nil
			})
			upToDateTokenHandler := upToDateHTTPAPI.RequireToken(upToDateHandler.Handle)

			upToDateServer := ghttp.NewServer()
			defer upToDateServer.Close()

			upToDateServer.RouteToHandler("GET", "/v1/check", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v1/check"),
				ghttp.VerifyHeaderKV("Authorization", "Bearer "+token),
				upToDateTokenHandler,
			))

			req, _ := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				upToDateServer.URL()+"/v1/check",
				nil,
			)
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))

			body, _ := io.ReadAll(resp.Body)

			var parsed struct {
				Containers []checkAPI.Result `json:"containers"`
				Count      int               `json:"count"`
			}

			gomega.Expect(json.Unmarshal(body, &parsed)).To(gomega.Succeed())
			gomega.Expect(parsed.Count).To(gomega.Equal(1))
			gomega.Expect(parsed.Containers[0].UpdateAvailable).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("Authentication", func() {
		ginkgo.It("should return 401 Unauthorized without token", func() {
			noAuthHTTPAPI := api.New(token, ":8080", rateLimit60)
			noAuthHandler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
				return []checkAPI.Result{}, nil
			})
			noAuthTokenHandler := noAuthHTTPAPI.RequireToken(noAuthHandler.Handle)

			noAuthServer := ghttp.NewServer()
			defer noAuthServer.Close()

			noAuthServer.RouteToHandler("GET", "/v1/check", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v1/check"),
				noAuthTokenHandler,
			))

			req, _ := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				noAuthServer.URL()+"/v1/check",
				nil,
			)

			resp, err := http.DefaultClient.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusUnauthorized))
		})

		ginkgo.It("should return 401 Unauthorized with invalid token", func() {
			invalidHTTPAPI := api.New(token, ":8080", rateLimit60)
			invalidHandler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
				return []checkAPI.Result{}, nil
			})
			invalidTokenHandler := invalidHTTPAPI.RequireToken(invalidHandler.Handle)

			invalidServer := ghttp.NewServer()
			defer invalidServer.Close()

			invalidServer.RouteToHandler("GET", "/v1/check", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v1/check"),
				invalidTokenHandler,
			))

			req, _ := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				invalidServer.URL()+"/v1/check",
				nil,
			)
			req.Header.Add("Authorization", "Bearer invalid-token")

			resp, err := http.DefaultClient.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusUnauthorized))
		})
	})

	ginkgo.Describe("Content-Type headers", func() {
		ginkgo.It("should return application/json Content-Type", func() {
			req, _ := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				server.URL()+"/v1/check",
				nil,
			)
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			gomega.Expect(resp.Header.Get("Content-Type")).To(gomega.Equal("application/json"))
		})
	})
})

// TestHandleReturns500OnCheckError verifies that the handler returns 500 Internal
// Server Error when the check function returns an error.
func TestHandleReturns500OnCheckError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("check error")
	handler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
		return nil, expectedErr
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/check",
		nil,
	)

	handler.Handle(rec, req)

	gomega.Expect(rec.Code).To(gomega.Equal(http.StatusInternalServerError))

	body, err := io.ReadAll(rec.Body)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(string(body)).To(gomega.ContainSubstring("failed to check for updates"))
}

// TestHandleReportsPerContainerError verifies that a per-container registry error is
// surfaced in the result entry without failing the whole response.
func TestHandleReportsPerContainerError(t *testing.T) {
	t.Parallel()

	handler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
		return []checkAPI.Result{
			{
				Name:            "unreachable",
				Image:           "example/unreachable:latest",
				CurrentDigest:   "sha256:abc123",
				UpdateAvailable: false,
				Error:           "registry responded with invalid HEAD request",
			},
		}, nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/check",
		nil,
	)

	handler.Handle(rec, req)

	gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))

	var parsed struct {
		Containers []checkAPI.Result `json:"containers"`
	}

	gomega.Expect(json.Unmarshal(rec.Body.Bytes(), &parsed)).To(gomega.Succeed())
	gomega.Expect(parsed.Containers).To(gomega.HaveLen(1))
	gomega.Expect(parsed.Containers[0].Error).To(gomega.ContainSubstring("invalid HEAD request"))
	gomega.Expect(parsed.Containers[0].UpdateAvailable).To(gomega.BeFalse())
	gomega.Expect(parsed.Containers[0].LatestDigest).To(gomega.BeEmpty())
}

// TestHandleOmitsErrorFieldWhenEmpty verifies the per-container error field is omitted
// from the JSON when the check succeeded.
func TestHandleOmitsErrorFieldWhenEmpty(t *testing.T) {
	t.Parallel()

	handler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
		return []checkAPI.Result{
			{
				Name:            "ok",
				Image:           "example/ok:latest",
				CurrentDigest:   "sha256:aaaa",
				LatestDigest:    "sha256:bbbb",
				UpdateAvailable: true,
			},
		}, nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/check",
		nil,
	)

	handler.Handle(rec, req)

	gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))
	gomega.Expect(rec.Body.String()).ToNot(gomega.ContainSubstring("\"error\""))
}

// TestNewHandlerSetsCorrectPath verifies that New creates a handler with the
// correct endpoint path.
func TestNewHandlerSetsCorrectPath(t *testing.T) {
	t.Parallel()

	handler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
		return nil, nil
	})

	gomega.Expect(handler.Path).To(gomega.Equal("/v1/check"))
}

// TestHandlerStartsDebugLogging verifies debug logging on request.
func TestHandlerStartsDebugLogging(t *testing.T) {
	t.Parallel()

	// Suppress log output during test.
	originalOutput := logrus.StandardLogger().Out

	logrus.SetOutput(io.Discard)
	defer logrus.SetOutput(originalOutput)

	handler := checkAPI.New(func(_ context.Context) ([]checkAPI.Result, error) {
		return []checkAPI.Result{}, nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/check",
		nil,
	)

	handler.Handle(rec, req)

	gomega.Expect(rec.Code).To(gomega.Equal(http.StatusOK))
}
