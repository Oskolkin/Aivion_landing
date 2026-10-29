package chatexport

import "errors"

// ErrExportStorageUnavailable indicates the exports bucket/object store is unreachable.
var ErrExportStorageUnavailable = errors.New("export storage unavailable")

func wrapStorageError(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(ErrExportStorageUnavailable, err)
}

// WrapStorageDownloadError marks download failures from the exports bucket.
func WrapStorageDownloadError(err error) error {
	return wrapStorageError(err)
}
