package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/elwinar/rcoredump"
	"github.com/inconshreveable/log15"
	"github.com/rs/xid"
)

type indexRequest struct {
	log   log15.Logger
	r     *http.Request
	dir   string
	index Index

	err error
	uid string
	rcoredump.IndexRequest
	body   *bufio.Reader
	reader *gzip.Reader
}

func (r *indexRequest) init() {
	r.uid = xid.New().String()
	r.log = r.log.New("uid", r.uid)
	r.body = bufio.NewReader(r.r.Body)
}

func (r *indexRequest) close() {
	if r.reader != nil {
		r.reader.Close()
	}
	io.Copy(ioutil.Discard, r.r.Body)
	r.r.Body.Close()
}

func (r *indexRequest) prepareReader() error {
	var err error
	if r.reader == nil {
		r.reader, err = gzip.NewReader(r.body)
	} else {
		err = r.reader.Reset(r.body)
	}
	if err != nil {
		return err
	}
	r.reader.Multistream(false)
	return nil
}

func (r *indexRequest) read() {
	if r.err != nil {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	err = json.NewDecoder(r.reader).Decode(&r.IndexRequest)
	if err != nil {
		r.err = wrap(err, "parsing header")
		return
	}
}

func (r *indexRequest) readCore() {
	if r.err != nil {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	f, err := os.Create(corepath(r.dir, r.uid))
	if err != nil {
		r.err = wrap(err, "creating core file")
		return
	}
	defer f.Close()

	_, err = io.Copy(f, r.reader)
	if err != nil {
		r.err = wrap(err, "reading core")
		return
	}
}

func (r *indexRequest) readExecutable() {
	if r.err != nil {
		return
	}

	err := r.prepareReader()
	if err != nil {
		r.err = wrap(err, "preparing gzip reader")
		return
	}

	f, err := os.Create(exepath(r.dir, r.ExecutableHash))
	if err != nil {
		r.err = wrap(err, "creating executable file")
		return
	}
	defer f.Close()

	_, err = io.Copy(f, r.reader)
	if err != nil {
		r.err = wrap(err, "writing executable file")
		return
	}
}

func (r *indexRequest) indexCore() {
	if r.err != nil {
		return
	}

	err := r.index.Index(Coredump{
		UID:            r.uid,
		Date:           r.Date,
		Hostname:       r.Hostname,
		ExecutablePath: r.ExecutablePath,
		ExecutableHash: r.ExecutableHash,
		Metadata:       r.Metadata,
		Analyzed:       false,
	})
	if err != nil {
		r.err = wrap(err, "indexing core")
		return
	}
}
