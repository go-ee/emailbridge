# NAME

emailbridge - Email Bridge CLI

# SYNOPSIS

emailbridge

```
[--debug]
[--help|-h]
[--senderEmail]=[value]
[--senderPassword]=[value]
[--version|-v]
```

**Usage**:

```
emailbridge [GLOBAL OPTIONS] command [COMMAND OPTIONS] [ARGUMENTS...]
```

# GLOBAL OPTIONS

**--debug**: Enable debug log level

**--help, -h**: show help

**--senderEmail**="": Sender Email for authentication and FROM field

**--senderPassword**="": Sender password for authentication 

**--version, -v**: print the version


# COMMANDS

## startBridge

Start HTTP to EMAIL Bridge

**--encryptPassphrase**="": 

**--pathStatic**="":  (default: static)

**--pathStorage**="":  (default: storage)

**--port**="": HTTP Server port (default: 8080)

## sendExampleMessage

Send example message

## help, h
