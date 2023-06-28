PROGSRC=txtpages.go editwords.go dbdata.go
LIBSRC=db.go util.go web.go

all: txtpages t

dep:
	go env -w GO111MODULE=auto
	go get github.com/mattn/go-sqlite3
	go get github.com/yuin/goldmark

txtpages: $(PROGSRC) $(LIBSRC)
	go build -o txtpages $(PROGSRC) $(LIBSRC)

t: t.go util.go
	go build -o t t.go util.go

clean:
	rm -rf txtpages t

