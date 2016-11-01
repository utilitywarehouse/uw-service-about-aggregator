FROM alpine:3.3

ADD *.go /uw-aggregate-about/
ADD *.html /

RUN apk add --update bash \
  && apk --update add git bzr \
  && apk --update add go \
  && export GOPATH=/gopath \
  && REPO_PATH="github.com/utilitywarehouse/uw-aggregate-about" \
  && mkdir -p $GOPATH/src/${REPO_PATH} \
  && mv uw-aggregate-about/* $GOPATH/src/${REPO_PATH} \
  && rm -rf uw-aggregate-about \
  && cd $GOPATH/src/${REPO_PATH} \
  && go get -t ./... \
  && go build \
  && mv uw-aggregate-about /uw-aggregate-about \
  && apk del go git bzr \
  && rm -rf $GOPATH /var/cache/apk/*

CMD [ "/uw-aggregate-about" ]