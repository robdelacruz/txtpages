all: t

dep:
	go get -u github.com/mattn/go-sqlite3
	go get -u golang.org/x/crypto/bcrypt

t: t.go
	go build -o t t.go

clean:
	rm -rf t

