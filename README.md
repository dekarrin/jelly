# jelly

Go server framework to run projects that need websites and to learn building
servers with Go.

It grew out of the TunaQuest server.

This readme will be updated with future releases.

## Dev Dependencies

In general, Jelly can be developed on with no tools beyond whatever environment
you use for Go code.

The exception is changing the mocks. Some places in Jelly use interfaces to
decouple certain packages from each other, and their code is mocked by using the
gomock package. The mocks are pre-generated, but will need to be re-generated if
the interface they mock is modified.

To get the gomock package, simply run the following:

```
go install go.uber.org/mock/mockgen@latest
```

Then execute tools/scripts/mocks to create the mocks.