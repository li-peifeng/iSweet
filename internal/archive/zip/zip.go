package zip

import (
	"github.com/alist-org/alist/v3/internal/archive/tool"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/stream"
	"github.com/yeka/zip"
	"io"
	"os"
	stdpath "path"
	"strings"
)

type Zip struct {
}

func (_ *Zip) AcceptedExtensions() []string {
	return []string{".zip"}
}

func (_ *Zip) GetMeta(ss *stream.SeekableStream, args model.ArchiveArgs) (model.ArchiveMeta, error) {
	reader, err := stream.NewReadAtSeeker(ss, 0)
	if err != nil {
		return nil, err
	}
	zipReader, err := zip.NewReader(reader, ss.GetSize())
	if err != nil {
		return nil, err
	}
	encrypted := false
	for _, file := range zipReader.File {
		if file.IsEncrypted() {
			encrypted = true
			break
		}
	}
	return &model.ArchiveMetaInfo{
		Comment:   zipReader.Comment,
		Encrypted: encrypted,
	}, nil
}

func (_ *Zip) List(ss *stream.SeekableStream, args model.ArchiveInnerArgs) ([]model.Obj, error) {
	reader, err := stream.NewReadAtSeeker(ss, 0)
	if err != nil {
		return nil, err
	}
	zipReader, err := zip.NewReader(reader, ss.GetSize())
	if err != nil {
		return nil, err
	}
	if args.InnerPath == "/" {
		ret := make([]model.Obj, 0)
		passVerified := false
		for _, file := range zipReader.File {
			if !passVerified && file.IsEncrypted() {
				file.SetPassword(args.Password)
				rc, e := file.Open()
				if e != nil {
					return nil, filterPassword(e)
				}
				_ = rc.Close()
				passVerified = true
			}
			name := decodeName(file.Name)
			if strings.Contains(strings.TrimSuffix(name, "/"), "/") {
				continue
			}
			ret = append(ret, toModelObj(file.FileInfo()))
		}
		return ret, nil
	} else {
		innerPath := strings.TrimPrefix(args.InnerPath, "/") + "/"
		ret := make([]model.Obj, 0)
		exist := false
		for _, file := range zipReader.File {
			name := decodeName(file.Name)
			if name == innerPath {
				exist = true
			}
			dir := stdpath.Dir(strings.TrimSuffix(name, "/")) + "/"
			if dir != innerPath {
				continue
			}
			ret = append(ret, toModelObj(file.FileInfo()))
		}
		if !exist {
			return nil, errs.ObjectNotFound
		}
		return ret, nil
	}
}

func (_ *Zip) Extract(ss *stream.SeekableStream, args model.ArchiveInnerArgs) (io.ReadCloser, int64, error) {
	reader, err := stream.NewReadAtSeeker(ss, 0)
	if err != nil {
		return nil, 0, err
	}
	zipReader, err := zip.NewReader(reader, ss.GetSize())
	if err != nil {
		return nil, 0, err
	}
	innerPath := strings.TrimPrefix(args.InnerPath, "/")
	for _, file := range zipReader.File {
		if decodeName(file.Name) == innerPath {
			if file.IsEncrypted() {
				file.SetPassword(args.Password)
			}
			r, e := file.Open()
			if e != nil {
				return nil, 0, e
			}
			return r, file.FileInfo().Size(), nil
		}
	}
	return nil, 0, errs.ObjectNotFound
}

func (_ *Zip) Decompress(ss *stream.SeekableStream, outputPath string, args model.ArchiveInnerArgs, up model.UpdateProgress) error {
	reader, err := stream.NewReadAtSeeker(ss, 0)
	if err != nil {
		return err
	}
	zipReader, err := zip.NewReader(reader, ss.GetSize())
	if err != nil {
		return err
	}
	if args.InnerPath == "/" {
		for i, file := range zipReader.File {
			name := decodeName(file.Name)
			err = decompress(file, name, outputPath, args.Password)
			if err != nil {
				return err
			}
			up(float64(i+1) * 100.0 / float64(len(zipReader.File)))
		}
	} else {
		innerPath := strings.TrimPrefix(args.InnerPath, "/")
		innerBase := stdpath.Base(innerPath)
		createdBaseDir := false
		for _, file := range zipReader.File {
			name := decodeName(file.Name)
			if name == innerPath {
				err = _decompress(file, outputPath, args.Password, up)
				if err != nil {
					return err
				}
				break
			} else if strings.HasPrefix(name, innerPath+"/") {
				targetPath := stdpath.Join(outputPath, innerBase)
				if !createdBaseDir {
					err = os.Mkdir(targetPath, 0700)
					if err != nil {
						return err
					}
					createdBaseDir = true
				}
				restPath := strings.TrimPrefix(name, innerPath+"/")
				err = decompress(file, restPath, targetPath, args.Password)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

var _ tool.Tool = (*Zip)(nil)

func init() {
	tool.RegisterTool(&Zip{})
}
