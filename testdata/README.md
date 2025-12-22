# testdata

`test-0.1.0.tgz` is used by Go tests directly, while `test-0.2.0.tgz` is used by Dagger to prepare a repository to pass to the Go tests as an argument.

Both were generated via the following sequence of commands:

```sh
helm create test
cd test/
# sed -i 's/0.1.0/0.2.0/g'
helm package .
```
