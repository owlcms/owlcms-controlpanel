#!/bin/bash -x
export TAG=v1.6.4
export DEB_TAG=${TAG#v}
git pull
git commit -am "Release $TAG prepared."
git tag -a ${TAG} -m "Release $TAG"
git push origin --tags
