FROM t0twn/start AS build
FROM qbittorrentofficial/qbittorrent-nox
COPY --from=build /usr/local/bin/start /usr/local/bin/start
ENTRYPOINT ["start", "qbittorrent-nox"]
