#!/usr/bin/env bash

go build && docker build . --tag bclouser/ota-kube && docker push bclouser/ota-kube


