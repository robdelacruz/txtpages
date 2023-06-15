LIBSRC=db.go util.go web.go

all: joti

dep:
	go env -w GO111MODULE=auto
	go get github.com/mattn/go-sqlite3

joti: joti.go
	go build -o joti joti.go $(LIBSRC)

clean:
	rm -rf joti

