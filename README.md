# Magereport

Report changes in Magento config variables.

## Example

    $ cd magento
    $ snapshot take

    ... make some changes to config ...

    $ snapshot list
      1  snapshot...
      2  snapshot...
    ... list of snapshot
    $ snapshot diff 1 2

    ... check the result

    $ snapshot export 1 2 > new-feature-set-config.magerun

    ... on the server

    $ magerun script < new-feature-set-config.magerun

