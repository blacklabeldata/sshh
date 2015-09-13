## SSH Echo Shell

This example shows how a custom SSH shell could be implemented.

### Running the example

To run example, type:

```
go run *.go
```

The server should now be started. Next, try to login:

```
$ ssh admin@127.0.0.1 -p 9022
```

The password is `password`...

#### Increased logging

To increase the logging level set this env variable:

```
export LOGXI=*=DBG
```

Then run the server again.
