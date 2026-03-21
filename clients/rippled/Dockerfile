ARG baseimage=rippleci/rippled
ARG tag=develop

FROM --platform=linux/amd64 $baseimage:$tag

# Install python3 for validators.json conversion (RHEL-based image).
RUN dnf install -y python3 && dnf clean all || true

# Write version.
RUN rippled --version > /version.txt 2>&1 || echo "rippled-unknown" > /version.txt

# Create config and data directories.
RUN mkdir -p /etc/rippled /var/lib/rippled/db /hive-bin

# Add entry script.
ADD xrpl_start.sh /xrpl_start.sh
RUN chmod +x /xrpl_start.sh

EXPOSE 5005 6006 51235

ENTRYPOINT ["/xrpl_start.sh"]
