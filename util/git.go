package util

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func CloneRepo(path string, option *git.CloneOptions, ctx context.Context) (*git.Repository, error) {
	return git.PlainCloneContext(ctx,
		path,
		false,
		option,
	)
}

func CheckOutHash(repository *git.Repository, hash string) error {
	if !plumbing.IsHash(hash) {
		return fmt.Errorf("%s Is Not Hash", hash)
	}
	options := &git.CheckoutOptions{
		Force: true,
	}
	options.Hash = plumbing.NewHash(hash)
	return CheckOut(repository, options)
}

func CheckOut(repository *git.Repository, option *git.CheckoutOptions) error {
	worktree, err := repository.Worktree()
	if err != nil {
		return err
	}
	err = worktree.Checkout(option)
	if err != nil {
		return err
	}
	return nil
}

func GetLogsHash(repository *git.Repository, hash string) (object.CommitIter, error) {
	h := plumbing.NewHash(hash)
	options := &git.LogOptions{From: h}
	return GetLogs(repository, options)
}

func GetLogs(repository *git.Repository, option *git.LogOptions) (object.CommitIter, error) {
	return repository.Log(option)
}
