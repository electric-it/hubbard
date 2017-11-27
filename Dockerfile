
FROM buildpack-deps:jessie
ARG GITHUB_URL
ARG GITHUB_ACCESS_TOKEN
ADD ./pkg/linux-amd64/hubbard .
ADD ./register-hubbard-service .
ADD ./hubbard-service-def .
RUN ./register-hubbard-service
CMD bash
