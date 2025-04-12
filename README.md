# Multiplayer

```
             _____________________________________________________
            /                       snapshot                      \
udp.Listener -> udp.Mux -> InputQueue -> Simulation -> SnapshotQueue
           \____________/
                 ack
```

## Development

1. [Install ebitengine dependencies][ebitengine_install].

2. Run the server:

   ```bash
   go run ./cmd/server
   ```

3. Run `n` clients (`n` > 1 is experimental):

   ```bash
   ./spawn_clients.sh <n> # replace <n> with the desired number of clients
   ```

[ebitengine_install]: https://ebitengine.org/en/documents/install
[#2]: https://github.com/utilyre/multiplayer/pull/2

## Resources

- [Networked Physics](https://gafferongames.com/categories/networked-physics)
