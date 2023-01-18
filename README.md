# WebSocket implementation for Go.

_This project is WIP_

I started this project out of curiosity how the WebSocket protocol is working under the hood. I really hope that I'll be able to finish it so that it could be used as a WebSocket library for Go in the feature.

## Project status

Current goal is to fully and correctly support upgrading and closing the connection and exchanging data in text format between client and the server (OpText).

- Read RFC again and check if upgrading the connection works properly
- Look into extensions and how they work (is it hard to support them?)
- Work on error handling, to report descriptive errors to api users
- Write tests (how to even test things like that? Maybe it requires understanding how net.Conn is implemented?)

### Upgrading HTTP to WebSocket

- [x] Checking handshake
- [x] Checking origin
- [x] Retrieving Sec-WebSocket-Key and generating access key
- [x] Resolving protocol
- [x] Checking for data sent after opening handshake but
      before getting the opening confirmation
- [ ] Extensions (and everything related with them)

### Communication with the client

- [x] Decoding incoming frames (getting fin, rsv1, rsv2, rsv3, opcode, mask, etc.)
- [x] Unmasking payload
- [ ] Writing data to the client (add support for everything described in RFC)
- [ ] Add support for different opcodes
- [ ] Closing the connection (make sure that it works as expected)
- [ ] Working with buffers (need to do more reaserch on that. How, when and if should I use them for reading / writing data)
- [ ] Fragmentation (only not fragmented frames are supported right now)

I hope this todo list will become clearer with time as I'll understand each part more deeply.
