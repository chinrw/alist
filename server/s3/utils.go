// Credits: https://pkg.go.dev/github.com/rclone/rclone@v1.65.2/cmd/serve/s3
// Package s3 implements a fake s3 server for alist
package s3

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/fs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/internal/setting"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/alist-org/gofakes3"
	log "github.com/sirupsen/logrus"
)

type Bucket struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func getAndParseBuckets() ([]Bucket, error) {
	var res []Bucket
	err := json.Unmarshal([]byte(setting.GetStr(conf.S3Buckets)), &res)
	return res, err
}

func getBucketByName(name string) (Bucket, error) {
	buckets, err := getAndParseBuckets()
	if err != nil {
		return Bucket{}, err
	}
	for _, b := range buckets {
		if b.Name == name {
			return b, nil
		}
	}
	return Bucket{}, gofakes3.BucketNotFound(name)
}

func getDirEntries(path string) ([]model.Obj, error) {
	ctx := context.Background()
	meta, _ := op.GetNearestMeta(path)
	fi, err := fs.Get(context.WithValue(ctx, "meta", meta), path, &fs.GetArgs{})
	if errs.IsNotFoundError(err) {
		return nil, gofakes3.ErrNoSuchKey
	} else if err != nil {
		return nil, gofakes3.ErrNoSuchKey
	}

	if !fi.IsDir() {
		return nil, gofakes3.ErrNoSuchKey
	}

	dirEntries, err := fs.List(context.WithValue(ctx, "meta", meta), path, &fs.ListArgs{})
	if err != nil {
		return nil, err
	}

	return dirEntries, nil
}

func getFileHashByte(node model.Obj) []byte {
	b, err := hex.DecodeString(getFileHash(node))
	if err != nil {
		return nil
	}
	return b
}

func getFileHash(node model.Obj) string {
	// var o fs.Object

	hash_data := node.GetHash()
	var hash string

	// Try MD5 first
	hash = hash_data.GetHash(utils.MD5)
	if hash != "" {
		log.Infof("Using MD5 hash: %s", hash)
		return hash
	}

	// If MD5 is not available, try SHA1
	hash = hash_data.GetHash(utils.SHA1)
	if hash != "" {
		log.Infof("Using sha1 hash: %s", hash)
		return hash
	}

	// If SHA1 is not available, try SHA256
	hash = hash_data.GetHash(utils.SHA256)
	if hash != "" {
		log.Infof("Using sha256 hash: %s", hash)
		return hash
	}

	// If none of the hashes are available, return an empty string or handle accordingly
	return ""
}

func prefixParser(p *gofakes3.Prefix) (path, remaining string) {
	idx := strings.LastIndexByte(p.Prefix, '/')
	if idx < 0 {
		return "", p.Prefix
	}
	return p.Prefix[:idx], p.Prefix[idx+1:]
}

// // FIXME this could be implemented by VFS.MkdirAll()
// func mkdirRecursive(path string, VFS *vfs.VFS) error {
// 	path = strings.Trim(path, "/")
// 	dirs := strings.Split(path, "/")
// 	dir := ""
// 	for _, d := range dirs {
// 		dir += "/" + d
// 		if _, err := VFS.Stat(dir); err != nil {
// 			err := VFS.Mkdir(dir, 0777)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }

// func rmdirRecursive(p string, VFS *vfs.VFS) {
// 	dir := path.Dir(p)
// 	if !strings.ContainsAny(dir, "/\\") {
// 		// might be bucket(root)
// 		return
// 	}
// 	if _, err := VFS.Stat(dir); err == nil {
// 		err := VFS.Remove(dir)
// 		if err != nil {
// 			return
// 		}
// 		rmdirRecursive(dir, VFS)
// 	}
// }

func authlistResolver() map[string]string {
	s3accesskeyid := setting.GetStr(conf.S3AccessKeyId)
	s3secretaccesskey := setting.GetStr(conf.S3SecretAccessKey)
	if s3accesskeyid == "" && s3secretaccesskey == "" {
		return nil
	}
	authList := make(map[string]string)
	authList[s3accesskeyid] = s3secretaccesskey
	return authList
}
