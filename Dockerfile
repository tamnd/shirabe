# Consumed by GoReleaser: it copies the already cross-compiled binary out of the
# build context rather than compiling, so the image build is fast and uses the
# same static binary every other artifact ships.
#
# shirabe is a pure-Go server with no runtime dependency beyond CA roots, so the
# image is a minimal Alpine with ca-certificates and tzdata. Exec sources need
# their CLIs on PATH; this image ships none, so a container serves the native
# sources unless you mount or extend it with the tools you want.
#
# GoReleaser builds one multi-platform image with buildx and stages each
# platform's binary under a $TARGETPLATFORM directory (e.g. linux/amd64/) in the
# build context, so the COPY line selects the right one through the automatic
# TARGETPLATFORM build arg.
FROM alpine:3.21

ARG TARGETPLATFORM

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 shirabe \
 && mkdir -p /data \
 && chown shirabe:shirabe /data

COPY $TARGETPLATFORM/shirabe /usr/bin/shirabe

USER shirabe
WORKDIR /data

# User manifests are read from ~/.config/shirabe/sources.d; mount a volume at
# /data to add your own:
#
#   docker run -p 8879:8879 -v "$PWD/shirabe:/data" ghcr.io/tamnd/shirabe
#
ENV HOME=/data

EXPOSE 8879

VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/shirabe"]
CMD ["serve", "--addr", ":8879"]
