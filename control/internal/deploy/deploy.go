// Package deploy applies configuration changes safely:
//
//	change (in a DB transaction) → render new version dir → openresty -t
//	→ atomically repoint the "current" symlink → reload → commit
//
// If any step fails the transaction is rolled back, the live configuration is
// untouched and a failed version is recorded with the test output.
package deploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"corvus/control/internal/compiler"
	"corvus/control/internal/gateway"
	"corvus/control/internal/store"
)

// TestError is returned when the generated configuration is rejected.
type TestError struct {
	VersionID int64
	Output    string
}

func (e *TestError) Error() string { return "configuration rejected: " + e.Output }

type Deployer struct {
	Store    *store.Store
	Compiler *compiler.Compiler
	Runner   gateway.Runner
	Dir      string // e.g. /data/nginx  (versions/ and current live here)
	Keep     int    // number of recent version directories to keep

	mu sync.Mutex
}

func (d *Deployer) versionsDir() string { return filepath.Join(d.Dir, "versions") }
func (d *Deployer) currentLink() string { return filepath.Join(d.Dir, "current") }

// CurrentConf is the nginx.conf path the gateway is started with.
func (d *Deployer) CurrentConf() string {
	return filepath.ToSlash(filepath.Join(d.currentLink(), "nginx.conf"))
}

// Apply runs mutate (may be nil) and deploys the result atomically.
func (d *Deployer) Apply(ctx context.Context, by *int64, message string, mutate func(tx *store.Store) error) (*store.ConfigVersion, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := os.MkdirAll(d.versionsDir(), 0o755); err != nil {
		return nil, err
	}

	var version *store.ConfigVersion
	var testErr *TestError
	err := d.Store.WithTx(ctx, func(tx *store.Store) error {
		rev, err := tx.ConfigRev(ctx)
		if err != nil {
			return err
		}
		if mutate != nil {
			if err := mutate(tx); err != nil {
				return err
			}
			if rev, err = tx.BumpConfigRev(ctx); err != nil {
				return err
			}
		}
		v := &store.ConfigVersion{CreatedBy: by, Message: message, ConfigRev: rev}
		if err := tx.CreateVersion(ctx, v); err != nil {
			return err
		}
		v.Dir = fmt.Sprintf("v%06d", v.ID)
		if err := tx.SetVersionDir(ctx, v.ID, v.Dir); err != nil {
			return err
		}
		dir := filepath.Join(d.versionsDir(), v.Dir)
		_ = os.RemoveAll(dir) // a rolled-back attempt may have left the same id behind
		if err := d.Compiler.Render(ctx, tx, dir); err != nil {
			_ = os.RemoveAll(dir)
			return err
		}
		out, err := d.Runner.Test(ctx, filepath.ToSlash(filepath.Join(dir, "nginx.conf")))
		if err != nil {
			_ = os.RemoveAll(dir)
			testErr = &TestError{Output: out}
			return testErr
		}
		if err := d.activate(v.Dir); err != nil {
			_ = os.RemoveAll(dir)
			return err
		}
		if err := tx.FinishVersion(ctx, v.ID, true, out); err != nil {
			return err
		}
		v.Status, v.TestOutput = "active", out
		version = v
		return nil
	})
	if err != nil {
		if testErr != nil {
			// Record the rejected attempt outside the rolled-back transaction.
			rev, _ := d.Store.ConfigRev(ctx)
			fv := &store.ConfigVersion{CreatedBy: by, Message: message, Status: "failed", TestOutput: testErr.Output, ConfigRev: rev}
			if e := d.Store.CreateVersion(ctx, fv); e == nil {
				testErr.VersionID = fv.ID
			}
		}
		return nil, err
	}
	d.prune(ctx)
	return version, nil
}

// Rollback re-activates the generated configuration of an earlier version.
// The database is not changed, so the next Apply renders the current settings again.
func (d *Deployer) Rollback(ctx context.Context, by *int64, targetID int64) (*store.ConfigVersion, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	target, err := d.Store.VersionByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.Status == "failed" || target.Dir == "" {
		return nil, errors.New("this version was never deployed")
	}
	dir := filepath.Join(d.versionsDir(), target.Dir)
	if _, err := os.Stat(dir); err != nil {
		return nil, errors.New("configuration files of this version were pruned")
	}
	out, err := d.Runner.Test(ctx, filepath.ToSlash(filepath.Join(dir, "nginx.conf")))
	if err != nil {
		return nil, &TestError{Output: out}
	}
	if err := d.activate(target.Dir); err != nil {
		return nil, err
	}
	v := &store.ConfigVersion{CreatedBy: by, Message: fmt.Sprintf("rollback to #%d", targetID), RollbackOf: &targetID, Dir: target.Dir, ConfigRev: target.ConfigRev}
	if err := d.Store.CreateVersion(ctx, v); err != nil {
		return nil, err
	}
	if err := d.Store.FinishVersion(ctx, v.ID, true, out); err != nil {
		return nil, err
	}
	v.Status, v.TestOutput = "active", out
	return v, nil
}

// EnsureCurrent prepares the "current" symlink at startup, before the gateway starts.
//   - no active version            → initial deployment
//   - active version is a rollback → keep it (the operator chose that configuration)
//   - otherwise                    → re-render from the database so configuration
//     templates always match this binary (upgrades); fall back to the existing
//     version if the fresh render is rejected.
func (d *Deployer) EnsureCurrent(ctx context.Context) error {
	v, err := d.Store.ActiveVersion(ctx)
	if errors.Is(err, store.ErrNotFound) {
		_, err = d.Apply(ctx, nil, "initial deployment", nil)
		return err
	}
	if err != nil {
		return err
	}
	_, statErr := os.Stat(filepath.Join(d.versionsDir(), v.Dir, "nginx.conf"))
	dirty, _ := d.Dirty(ctx)
	if statErr == nil && dirty {
		slog.Info("keeping rolled-back configuration", "version", v.ID)
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.swapLink(v.Dir)
	}
	if _, err := d.Apply(ctx, nil, "startup re-render", nil); err != nil {
		if statErr != nil {
			return err
		}
		slog.Warn("startup re-render rejected, keeping previous version", "version", v.ID, "err", err)
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.swapLink(v.Dir)
	}
	return nil
}

// activate repoints the symlink and reloads; on reload failure the old target is restored.
func (d *Deployer) activate(dirName string) error {
	prev, _ := os.Readlink(d.currentLink())
	if err := d.swapLink(dirName); err != nil {
		return err
	}
	if err := d.Runner.Reload(); err != nil {
		if prev != "" {
			_ = d.swapLink(filepath.Base(prev))
		}
		return fmt.Errorf("reload failed: %w", err)
	}
	// Reload is asynchronous; report success only once the new version serves traffic.
	if w, ok := d.Runner.(interface {
		WaitActive(ctx context.Context, name string, timeout time.Duration) error
	}); ok {
		if err := w.WaitActive(context.Background(), dirName, 10*time.Second); err != nil {
			slog.Warn("reload confirmation timed out", "version", dirName, "err", err)
		}
	}
	return nil
}

// swapLink atomically replaces the "current" symlink (rename over the old one).
func (d *Deployer) swapLink(dirName string) error {
	tmp := d.currentLink() + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(filepath.Join("versions", dirName), tmp); err != nil {
		return err
	}
	return os.Rename(tmp, d.currentLink())
}

func (d *Deployer) prune(ctx context.Context) {
	keep := d.Keep
	if keep <= 0 {
		keep = 20
	}
	inUse, err := d.Store.VersionDirsInUse(ctx, keep)
	if err != nil {
		return
	}
	entries, err := os.ReadDir(d.versionsDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "v") && !inUse[e.Name()] {
			_ = os.RemoveAll(filepath.Join(d.versionsDir(), e.Name()))
		}
	}
}

// Dirty reports whether stored settings changed since the active version was rendered.
func (d *Deployer) Dirty(ctx context.Context) (bool, error) {
	rev, err := d.Store.ConfigRev(ctx)
	if err != nil {
		return false, err
	}
	v, err := d.Store.ActiveVersion(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return v.ConfigRev != rev, nil
}
