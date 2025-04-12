# Multiplayer

```
             _____________________________________________________
            /                       snapshot                      \
udp.Listener -> udp.Mux -> InputQueue -> Simulation -> SnapshotQueue
           \_________/
               ack
```

## Development

1. [Install ebitengine dependencies][ebitengine_install].

2. Run the server:

   ```bash
   go run ./cmd/server
   ```

3. Run the client:

   ```bash
   go run ./cmd/client
   ```

4. Potentially run more clients (experimental, see [#2][#2]):

[ebitengine_install]: https://ebitengine.org/en/documents/install
[#2]: https://github.com/utilyre/multiplayer/pull/2

## Resources

- [Networked Physics](https://gafferongames.com/categories/networked-physics)
- [Network Programming with Go](https://www.amazon.com/Network-Programming-Go-Adam-Woodbeck/dp/1718500882)
