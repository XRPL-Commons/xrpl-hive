ARG baseimage=xrpllabsofficial/xrpld
ARG tag=latest

FROM --platform=linux/amd64 $baseimage:$tag

# xrpllabsofficial image is Debian-based, runs as root, ships rippled
# release binaries with proper amd64/arm64 support and no rippleci dev
# noise. xrpl_start.sh uses pure shell, so we don't even need python3.

# Write version.
RUN rippled --version > /version.txt 2>&1 || echo "rippled-unknown" > /version.txt

# Create config and data directories.
RUN mkdir -p /etc/rippled /var/lib/rippled/db /hive-bin

# Copy the known amendment list for this version.
# When updating to a new rippled version, regenerate this file from the
# client's Amendments:DBG log output.
ADD all_amendments.txt /etc/rippled/all_amendments.txt

# Add entry script.
ADD xrpl_start.sh /xrpl_start.sh
RUN chmod +x /xrpl_start.sh

EXPOSE 5005 6006 51235

ENTRYPOINT ["/xrpl_start.sh"]
