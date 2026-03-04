package ethswitch

import (
	"context"
	"fmt"
	"io"
	"os"
)

// SFTPClient defines the contract for downloading EOD clearing files.
type SFTPClient interface {
	DownloadClearingFile(ctx context.Context, remotePath, localPath string) error
}

// LocalFileSFTP is a development stub that reads from the local filesystem.
// In production, this is replaced with a real SFTP client (e.g., pkg/sftp).
type LocalFileSFTP struct{}

func NewLocalFileSFTP() SFTPClient { return &LocalFileSFTP{} }

func (s *LocalFileSFTP) DownloadClearingFile(_ context.Context, remotePath, localPath string) error {
	src, err := os.Open(remotePath)
	if err != nil {
		return fmt.Errorf("opening clearing file %s: %w", remotePath, err)
	}
	defer src.Close()

	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("creating local file %s: %w", localPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copying clearing file: %w", err)
	}
	return nil
}

var _ SFTPClient = (*LocalFileSFTP)(nil)
