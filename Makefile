PROGSRC=joti.go editwords.go
LIBSRC=db.go util.go web.go

all: joti

dep:
	go env -w GO111MODULE=auto
	go get github.com/mattn/go-sqlite3

joti: $(PROGSRC) $(LIBSRC)
	go build -o joti $(PROGSRC) $(LIBSRC)

clean:
	rm -rf joti

