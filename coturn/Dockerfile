FROM t0twn/start AS build

FROM debian:bookworm-slim
RUN apt update && apt install -y coturn && apt clean && rm -rf /var/lib/apt/lists/*
RUN ln -s /usr/bin/bash /usr/bin/ash
COPY --from=build /usr/local/bin/start /usr/local/bin/start
ENTRYPOINT ["start", "coturn"]