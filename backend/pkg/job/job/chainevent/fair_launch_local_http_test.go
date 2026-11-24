package chainevent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestFairLaunchLocalHTTPServerAuthorityBoundaries(t *testing.T) {
	now := time.Now().UTC()
	capability := fairLaunchLocalE2ETestCapability()
	capability.seal.admission.RunID = "123e4567-e89b-42d3-a456-426614174000"
	capability.seal.admission.ExpiresAt = now.Add(time.Hour)
	options := FairLaunchLocalE2EHTTPOptions{BindAddress: "127.0.0.1:0", FrontendOrigin: "http://127.0.0.1:4173", AdminOrigin: "http://127.0.0.1:5666", Token: "f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0"}
	server, err := NewFairLaunchLocalE2EHTTPServer(capability, testGORMPointer(), testGORMPointer(), options, now)
	if err != nil {
		t.Fatalf("valid local HTTP server rejected: %v", err)
	}
	defer server.listener.Close()
	if server.authority.BaseURL == "" || server.authority.RunID != capability.seal.admission.RunID || server.authority.Token != options.Token || !server.authority.ExpiresAt.Equal(capability.seal.admission.ExpiresAt) {
		t.Fatalf("authority projection mismatch: %+v", server.authority)
	}
	for _, mutate := range []func(*FairLaunchLocalE2EHTTPOptions){
		func(o *FairLaunchLocalE2EHTTPOptions) { o.BindAddress = "localhost:0" },
		func(o *FairLaunchLocalE2EHTTPOptions) { o.FrontendOrigin = "http://localhost:4173" },
		func(o *FairLaunchLocalE2EHTTPOptions) { o.AdminOrigin = o.FrontendOrigin },
		func(o *FairLaunchLocalE2EHTTPOptions) { o.Token = "bad" },
	} {
		candidate := options
		mutate(&candidate)
		if rejected, rejectErr := NewFairLaunchLocalE2EHTTPServer(capability, testGORMPointer(), testGORMPointer(), candidate, now); rejectErr == nil {
			rejected.listener.Close()
			t.Fatal("invalid local HTTP authority accepted")
		}
	}
}

func TestFairLaunchLocalHTTPAuthorizationAndOriginFailClosed(t *testing.T) {
	now := time.Now().UTC()
	capability := fairLaunchLocalE2ETestCapability()
	capability.seal.admission.RunID = "123e4567-e89b-42d3-a456-426614174000"
	capability.seal.admission.ExpiresAt = now.Add(time.Hour)
	options := FairLaunchLocalE2EHTTPOptions{BindAddress: "127.0.0.1:0", FrontendOrigin: "http://127.0.0.1:4173", AdminOrigin: "http://127.0.0.1:5666", Token: "abababababababababababababababababababababababababababababababab"}
	server, err := NewFairLaunchLocalE2EHTTPServer(capability, testGORMPointer(), testGORMPointer(), options, now)
	if err != nil {
		t.Fatal(err)
	}
	defer server.listener.Close()
	request := httptest.NewRequest(http.MethodGet, server.authority.BaseURL+"/health", nil)
	request.Host = server.authority.BaseURL[len("http://"):]
	request.Header.Set("origin", options.FrontendOrigin)
	request.Header.Set("x-jc-fl-run-id", capability.seal.admission.RunID)
	request.Header.Set("x-jc-fl-capability", options.Token)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("access-control-allow-origin") != options.FrontendOrigin {
		t.Fatalf("valid request rejected: %d %s", recorder.Code, recorder.Body.String())
	}
	if !server.authorized(request, now.Add(31*time.Minute)) {
		t.Fatal("HTTP authority retained the removed 30-minute lifetime")
	}
	if server.authorized(request, capability.seal.admission.ExpiresAt) {
		t.Fatal("HTTP authority did not fail closed at the run admission expiry")
	}
	for _, patch := range []func(*http.Request){
		func(r *http.Request) { r.Header.Set("origin", "http://localhost:4173") },
		func(r *http.Request) { r.Header.Set("x-jc-fl-run-id", "other-run") },
		func(r *http.Request) { r.Header.Set("x-jc-fl-capability", "cd"+options.Token[2:]) },
		func(r *http.Request) { r.Host = "127.0.0.1:1" },
	} {
		candidate := request.Clone(context.Background())
		patch(candidate)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, candidate)
		if response.Code < 400 {
			t.Fatalf("invalid request accepted: %d", response.Code)
		}
	}
	adminOnClient := request.Clone(context.Background())
	adminOnClient.Header.Set("origin", options.AdminOrigin)
	adminOnClient.URL.Path = "/client/project/v1/detail"
	adminOnClient.URL.RawQuery = "projectID=101"
	adminResponse := httptest.NewRecorder()
	server.ServeHTTP(adminResponse, adminOnClient)
	if adminResponse.Code != http.StatusForbidden {
		t.Fatalf("admin origin crossed into client route: %d", adminResponse.Code)
	}
	frontendOnAdmin := request.Clone(context.Background())
	frontendOnAdmin.Header.Set("origin", options.FrontendOrigin)
	frontendOnAdmin.URL.Path = "/admin/v1/my/permissions"
	frontendResponse := httptest.NewRecorder()
	server.ServeHTTP(frontendResponse, frontendOnAdmin)
	if frontendResponse.Code != http.StatusForbidden {
		t.Fatalf("frontend origin crossed into admin route: %d", frontendResponse.Code)
	}
	headerOrigin := request.Clone(context.Background())
	headerOrigin.Header.Del("origin")
	headerOrigin.Header.Set("x-jc-fl-origin", options.FrontendOrigin)
	headerOriginResponse := httptest.NewRecorder()
	server.ServeHTTP(headerOriginResponse, headerOrigin)
	if headerOriginResponse.Code != http.StatusOK {
		t.Fatalf("admitted non-browser origin header rejected: %d", headerOriginResponse.Code)
	}
}

func testGORMPointer() *gorm.DB { return &gorm.DB{} }
