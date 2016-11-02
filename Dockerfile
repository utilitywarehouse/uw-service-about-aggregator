FROM alpine:3.4

ADD *.go /about-aggregator/
ADD *.html /

RUN apk add --update bash \
  && apk --update add git bzr \
  && apk --update add go \
  && export GOPATH=/gopath \
  && REPO_PATH="github.com/utilitywarehouse/about-aggregator" \
  && mkdir -p $GOPATH/src/${REPO_PATH} \
  && mv about-aggregator/* $GOPATH/src/${REPO_PATH} \
  && rm -rf about-aggregator \
  && cd $GOPATH/src/${REPO_PATH} \
  && go get -t ./... \
  && go build \
  && mv about-aggregator /about-aggregator \
  && apk del go git bzr \
  && rm -rf $GOPATH /var/cache/apk/*

CMD [ "/about-aggregator" ]