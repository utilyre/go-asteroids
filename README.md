# Multiplayer

> [!WARNING]
>
> This repository is a work-in-progress and subject to ongoing updates.

Multiplayer online game from scratch.

## Development

1. [Install ebitengine dependencies][ebitengine_install].

2. Run the server:

   ```bash
   go run ./cmd/server
   ```

3. In the server logs, there will be a line similar to the one below that
   telling you what address the server is bound to:

   ```
	 11:10:53 INF bound udp/mcp listener address=[ADDRESS]
	 ```

	 Take note of `[ADDRESS]`.

3. Run the following. Replace `<n>` with the number of clients you'd like to
   run and also replace `[ADDRESS]` with the one you took note of in the
   previous step.

   ```bash
   ./spawn_clients.sh <n> -remote=[ADDRESS]
   ```

[ebitengine_install]: https://ebitengine.org/en/documents/install

## Resources

- [Networked Physics](https://gafferongames.com/categories/networked-physics)
- [Sneaky Race Conditions and Granular Locks](https://blogtitle.github.io/sneaky-race-conditions-and-granular-locks)
- [Network Programming with Go](https://www.amazon.com/Network-Programming-Go-Adam-Woodbeck/dp/1718500882)
