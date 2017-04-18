FROM ubuntu:16.04

COPY jujud /bin/

RUN mkdir -p /var/lib/juju/agents/caasoperator-foo-0/
COPY caasoperator.conf /var/lib/juju/agents/caasoperator-foo-0/agent.conf

RUN mkdir -p /var/lib/juju/tools/caasoperator-foo-0/

ENTRYPOINT ["jujud"]

CMD ["caasoperator", "--caasoperator-name",  "foo/0",  "--log-to-stderr"]

