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

## Resources

- [Networked Physics](https://gafferongames.com/categories/networked-physics)
- [Sneaky Race Conditions and Granular Locks](https://blogtitle.github.io/sneaky-race-conditions-and-granular-locks)
- [Network Programming with Go](https://www.amazon.com/Network-Programming-Go-Adam-Woodbeck/dp/1718500882)
