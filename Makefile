PROGSRC=txtpage.go editwords.go dbdata.go
LIBSRC=db.go util.go web.go

all: txtpage t

dep:
	go env -w GO111MODULE=auto
	go get github.com/mattn/go-sqlite3
	go get github.com/yuin/goldmark

txtpage: $(PROGSRC) $(LIBSRC)
	go build -o txtpage $(PROGSRC) $(LIBSRC)

t: t.go util.go
	go build -o t t.go util.go

clean:
	rm -rf txtpage t

