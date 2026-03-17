package main

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"testing"
	"testing/fstest"

	"samebits.com/evidra/internal/store"
	testutil "samebits.com/evidra/internal/testutil"
	"samebits.com/evidra/pkg/evidence"
)

func TestRun_ArgocdControllerRequiresDatabase(t *testing.T) {
	t.Setenv("EVIDRA_API_KEY", "test-key")
	t.Setenv("EVIDRA_ARGOCD_CONTROLLER_ENABLED", "true")
	t.Setenv("DATABASE_URL", "")

	deps := testRunDeps(t)
	deps.newSigner = func(signerConfig) (evidence.Signer, error) {
		return testutil.TestSigner(t), nil
	}
	deps.setupPersistence = func(string) (persistenceResources, func(), error) {
		t.Fatal("setupPersistence should not be called when DATABASE_URL is missing")
		return persistenceResources{}, func() {}, nil
	}

	if got := runWithDeps([]string{}, deps); got != 1 {
		t.Fatalf("runWithDeps() = %d, want 1", got)
	}
}

func TestRun_ArgocdControllerRequiresSigner(t *testing.T) {
	t.Setenv("EVIDRA_API_KEY", "test-key")
	t.Setenv("EVIDRA_ARGOCD_CONTROLLER_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://db/test")

	deps := testRunDeps(t)
	deps.newSigner = func(signerConfig) (evidence.Signer, error) {
		return nil, nil
	}
	deps.setupPersistence = func(string) (persistenceResources, func(), error) {
		t.Fatal("setupPersistence should not be called when signer is missing")
		return persistenceResources{}, func() {}, nil
	}

	if got := runWithDeps([]string{}, deps); got != 1 {
		t.Fatalf("runWithDeps() = %d, want 1", got)
	}
}

func TestRun_StartsArgocdControllerWhenEnabled(t *testing.T) {
	t.Setenv("EVIDRA_API_KEY", "test-key")
	t.Setenv("EVIDRA_ARGOCD_CONTROLLER_ENABLED", "true")
	t.Setenv("EVIDRA_ARGOCD_APPLICATION_NAMESPACE", "argocd-apps")
	t.Setenv("EVIDRA_ARGOCD_TENANT_ID", "tenant-123")
	t.Setenv("EVIDRA_KUBECONFIG", "/tmp/test-kubeconfig")
	t.Setenv("DATABASE_URL", "postgres://db/test")

	deps := testRunDeps(t)
	deps.newSigner = func(signerConfig) (evidence.Signer, error) {
		return testutil.TestSigner(t), nil
	}
	deps.setupPersistence = func(string) (persistenceResources, func(), error) {
		return persistenceResources{
			EntryStore: &store.EntryStore{},
		}, func() {}, nil
	}

	signals := make(chan os.Signal, 1)
	deps.signalChan = func() <-chan os.Signal { return signals }

	runner := &fakeControllerRunner{started: make(chan struct{})}
	var gotConfig argocdControllerConfig
	deps.controllerFactory = func(cfg argocdControllerConfig, entryStore *store.EntryStore, signer evidence.Signer) (controllerRunner, error) {
		gotConfig = cfg
		if entryStore == nil {
			t.Fatal("controllerFactory received nil entryStore")
		}
		if signer == nil {
			t.Fatal("controllerFactory received nil signer")
		}
		return runner, nil
	}

	go func() {
		<-runner.started
		signals <- os.Interrupt
	}()

	if got := runWithDeps([]string{}, deps); got != 0 {
		t.Fatalf("runWithDeps() = %d, want 0", got)
	}
	if !runner.wasStarted {
		t.Fatal("controller runner was not started")
	}
	if gotConfig.ApplicationNamespace != "argocd-apps" {
		t.Fatalf("ApplicationNamespace = %q, want argocd-apps", gotConfig.ApplicationNamespace)
	}
	if gotConfig.TenantID != "tenant-123" {
		t.Fatalf("TenantID = %q, want tenant-123", gotConfig.TenantID)
	}
	if gotConfig.KubeconfigPath != "/tmp/test-kubeconfig" {
		t.Fatalf("KubeconfigPath = %q, want /tmp/test-kubeconfig", gotConfig.KubeconfigPath)
	}
}

func testRunDeps(t *testing.T) runDeps {
	t.Helper()

	serverStopped := make(chan struct{})

	return runDeps{
		newSigner: func(signerConfig) (evidence.Signer, error) {
			return testutil.TestSigner(t), nil
		},
		setupPersistence: func(string) (persistenceResources, func(), error) {
			return persistenceResources{}, func() {}, nil
		},
		controllerFactory: func(argocdControllerConfig, *store.EntryStore, evidence.Signer) (controllerRunner, error) {
			return &fakeControllerRunner{started: make(chan struct{})}, nil
		},
		uiFS: func() (fs.FS, error) {
			return fstest.MapFS{}, nil
		},
		signalChan: func() <-chan os.Signal {
			ch := make(chan os.Signal, 1)
			ch <- os.Interrupt
			return ch
		},
		listenAndServe: func(*http.Server) error {
			<-serverStopped
			return http.ErrServerClosed
		},
		shutdownServer: func(*http.Server, context.Context) error {
			close(serverStopped)
			return nil
		},
		logf: func(string, ...any) {},
	}
}

type fakeControllerRunner struct {
	started    chan struct{}
	wasStarted bool
}

func (f *fakeControllerRunner) Run(ctx context.Context) error {
	f.wasStarted = true
	close(f.started)
	<-ctx.Done()
	return nil
}
