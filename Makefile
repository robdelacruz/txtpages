LIBSRC=db.go util.go web.go

all: t

dep:
	go get -u github.com/mattn/go-sqlite3

t: t.go
	go build -o t t.go $(LIBSRC)

clean:
	rm -rf t

