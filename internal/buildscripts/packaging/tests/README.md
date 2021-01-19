# Package and Installer Tests

1. From the root of the repository, run the following command to build the
   pytest docker image and start the container in an interactive shell:
   ```
   $ make run-pytest
   ```
1. To run the package tests, execute the following command in the pytest
   container:
   ```
   $ pytest [PYTEST_OPTIONS] internal/buildscripts/packaging/tests/package_test.py
   ```
   The package tests require that the deb and rpm packages to be tested exist
   locally in `<repo_base_dir>/dist`.  See [here](../fpm/deb/README.md) and
   [here](../fpm/rpm/README.md) for how to build the deb and rpm packages.
1. To run the installer tests, execute the following command in the pytest
   container:
   ```
    $ pytest [PYTEST_OPTIONS] internal/buildscripts/packaging/tests/installer_test.py
   ```
