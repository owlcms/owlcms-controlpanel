#!/bin/bash -x
export TAG=v1.8.2
export DEB_TAG=${TAG#v}
git pull
git commit -am "owlcms-launcher $TAG"
git push
git tag -a ${TAG} -m "owlcms-launcher $TAG"
git push origin --tags