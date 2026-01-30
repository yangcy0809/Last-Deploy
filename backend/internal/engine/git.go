package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

var hex40 = regexp.MustCompile(`\A[0-9a-fA-F]{40}\z`)

func CloneRepo(ctx context.Context, url, ref, destDir string) error {
	if url == "" {
		return fmt.Errorf("git url is required")
	}
	if destDir == "" {
		return fmt.Errorf("dest dir is required")
	}

	// Try to open existing repo first
	repo, err := git.PlainOpen(destDir)
	if err == nil {
		// Repo exists, fetch latest changes
		if err := fetchRepo(ctx, repo); err != nil {
			// Fetch failed, fall back to fresh clone
			goto freshClone
		}
		return checkoutRef(repo, ref)
	}

freshClone:
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return err
	}

	repo, err = git.PlainCloneContext(ctx, destDir, false, &git.CloneOptions{
		URL: url,
	})
	if err != nil {
		return err
	}
	return checkoutRef(repo, ref)
}

func fetchRepo(ctx context.Context, repo *git.Repository) error {
	err := repo.FetchContext(ctx, &git.FetchOptions{
		Force: true,
	})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

func checkoutRef(repo *git.Repository, ref string) error {
	if ref == "" {
		return nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	if hex40.MatchString(ref) {
		return wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(ref)})
	}

	candidates := []plumbing.ReferenceName{
		plumbing.ReferenceName(ref),
		plumbing.NewRemoteReferenceName("origin", ref),
		plumbing.NewTagReferenceName(ref),
		plumbing.NewBranchReferenceName(ref),
	}
	for _, name := range candidates {
		r, err := repo.Reference(name, true)
		if err == nil {
			return wt.Checkout(&git.CheckoutOptions{Hash: r.Hash()})
		}
	}

	revCandidates := []string{
		ref,
		"origin/" + ref,
		"refs/remotes/origin/" + ref,
		"refs/tags/" + ref,
		"refs/heads/" + ref,
	}
	for _, rev := range revCandidates {
		h, err := repo.ResolveRevision(plumbing.Revision(rev))
		if err == nil {
			return wt.Checkout(&git.CheckoutOptions{Hash: *h})
		}
	}

	return fmt.Errorf("unknown git_ref: %q", ref)
}
