PROGSRC=joti.go editwords.go dbdata.go
LIBSRC=db.go util.go web.go

all: joti t

dep:
	go env -w GO111MODULE=auto
	go get github.com/mattn/go-sqlite3
	go get github.com/yuin/goldmark

joti: $(PROGSRC) $(LIBSRC)
	go build -o joti $(PROGSRC) $(LIBSRC)

t: t.go
	go build -o t t.go

clean:
	rm -rf joti t

