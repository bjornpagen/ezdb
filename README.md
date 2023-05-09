# EZDB

EZDB is a simple and easy-to-use key-value store based on LMDB, offering a convenient and efficient way to manage data in your Go applications. This library provides a user-friendly interface to interact with the underlying LDB database.

## Features
- Supports multiple databases in a single environment
- Customizable options such as number of readers, databases, and batch size
- Optional logger integration

## Installation

To install EZDB, run the following command:

```sh
go get https://github.com/bjornpagen/ezdb
```

## Usage

Import the library and create a new database client:

```go
import (
	"github.com/bjornpagen/ezdb"
)

// Create a new database client
db, err := ezdb.New("testdb")
if err != nil {
	fmt.Println("error:", err)
}
```

Create a reference to a new database:

```go
// Create a new reference
ref, err := ezdb.NewRef[string, string]("ref_id", db)
if err != nil {
	fmt.Println("error:", err)
}
```

Insert a key-value pair into the referenced database:

```go
// Insert key-value pair
key := "my_key"
value := "Hello, World"
if err = ref.Put(&key, &value); err != nil {
	fmt.Println("error:", err)
}

// Retrieve the value by key
valueOut, err := ref.Get(&key)
if err != nil {
	fmt.Println("error:", err)
}

fmt.Println("retrieved value:", *valueOut)
```

## License

This project is licensed under the [Zero-Clause BSD License](https://opensource.org/license/0bsd/).

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue for bug reports, feature requests, or other discussions.

If you have any questions or need help, please contact the project owner or maintainers.

Happy coding!
