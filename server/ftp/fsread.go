package ftp

import (
	"context"
	ftpserver "github.com/KirCute/ftpserverlib-pasvportmap"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/fs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/internal/stream"
	"github.com/alist-org/alist/v3/pkg/http_range"
	"github.com/alist-org/alist/v3/server/common"
	"github.com/pkg/errors"
	"io"
	fs2 "io/fs"
	"net/http"
	"os"
	"time"
)

type FileDownloadProxy struct {
	ftpserver.FileTransfer
	ss     *stream.SeekableStream
	reader io.Reader
	cur    int64
}

func OpenDownload(ctx context.Context, reqPath string, offset int64) (*FileDownloadProxy, error) {
	user := ctx.Value("user").(*model.User)
	meta, err := op.GetNearestMeta(reqPath)
	if err != nil {
		if !errors.Is(errors.Cause(err), errs.MetaNotFound) {
			return nil, err
		}
	}
	ctx = context.WithValue(ctx, "meta", meta)
	if !common.CanAccess(user, meta, reqPath, ctx.Value("meta_pass").(string)) {
		return nil, errs.PermissionDenied
	}

	// directly use proxy
	header := *(ctx.Value("proxy_header").(*http.Header))
	link, obj, err := fs.Link(ctx, reqPath, model.LinkArgs{
		IP:     ctx.Value("client_ip").(string),
		Header: header,
	})
	if err != nil {
		return nil, err
	}
	fileStream := stream.FileStream{
		Obj: obj,
		Ctx: ctx,
	}
	ss, err := stream.NewSeekableStream(fileStream, link)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if offset != 0 {
		reader, err = ss.RangeRead(http_range.Range{Start: offset, Length: -1})
		if err != nil {
			return nil, err
		}
	} else {
		reader = ss
	}
	return &FileDownloadProxy{ss: ss, reader: reader}, nil
}

func (f *FileDownloadProxy) Read(p []byte) (n int, err error) {
	n, err = f.reader.Read(p)
	f.cur += int64(n)
	return n, err
}

func (f *FileDownloadProxy) Write(p []byte) (n int, err error) {
	return 0, errs.NotSupport
}

func (f *FileDownloadProxy) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		break
	case io.SeekCurrent:
		offset += f.cur
		break
	case io.SeekEnd:
		offset += f.ss.GetSize()
		break
	default:
		return 0, errs.NotSupport
	}
	if offset < 0 {
		return 0, errors.New("Seek: negative position")
	}
	reader, err := f.ss.RangeRead(http_range.Range{Start: offset, Length: -1})
	if err != nil {
		return f.cur, err
	}
	f.cur = offset
	f.reader = reader
	return offset, nil
}

func (f *FileDownloadProxy) Close() error {
	return f.ss.Close()
}

type OsFileInfoAdapter struct {
	obj model.Obj
}

func (o *OsFileInfoAdapter) Name() string {
	return o.obj.GetName()
}

func (o *OsFileInfoAdapter) Size() int64 {
	return o.obj.GetSize()
}

func (o *OsFileInfoAdapter) Mode() fs2.FileMode {
	var mode fs2.FileMode = 0755
	if o.IsDir() {
		mode |= fs2.ModeDir
	}
	return mode
}

func (o *OsFileInfoAdapter) ModTime() time.Time {
	return o.obj.ModTime()
}

func (o *OsFileInfoAdapter) IsDir() bool {
	return o.obj.IsDir()
}

func (o *OsFileInfoAdapter) Sys() any {
	return o.obj
}

func Stat(ctx context.Context, path string) (os.FileInfo, error) {
	user := ctx.Value("user").(*model.User)
	reqPath, err := user.JoinPath(path)
	if err != nil {
		return nil, err
	}
	meta, err := op.GetNearestMeta(reqPath)
	if err != nil {
		if !errors.Is(errors.Cause(err), errs.MetaNotFound) {
			return nil, err
		}
	}
	ctx = context.WithValue(ctx, "meta", meta)
	if !common.CanAccess(user, meta, reqPath, ctx.Value("meta_pass").(string)) {
		return nil, errs.PermissionDenied
	}
	obj, err := fs.Get(ctx, reqPath, &fs.GetArgs{})
	if err != nil {
		return nil, err
	}
	return &OsFileInfoAdapter{obj: obj}, nil
}

func List(ctx context.Context, path string) ([]os.FileInfo, error) {
	user := ctx.Value("user").(*model.User)
	reqPath, err := user.JoinPath(path)
	if err != nil {
		return nil, err
	}
	meta, err := op.GetNearestMeta(reqPath)
	if err != nil {
		if !errors.Is(errors.Cause(err), errs.MetaNotFound) {
			return nil, err
		}
	}
	ctx = context.WithValue(ctx, "meta", meta)
	if !common.CanAccess(user, meta, reqPath, ctx.Value("meta_pass").(string)) {
		return nil, errs.PermissionDenied
	}
	objs, err := fs.List(ctx, reqPath, &fs.ListArgs{})
	if err != nil {
		return nil, err
	}
	ret := make([]os.FileInfo, len(objs))
	for i, obj := range objs {
		ret[i] = &OsFileInfoAdapter{obj: obj}
	}
	return ret, nil
}
