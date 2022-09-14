# Adding domains @automatic-domains-registration
Assuming you've been using the [Web Console Search][web-console-search]
interface, you're likely to have already registered a few domains.

Ownership of the domains has been validating, e.g. by dropping
a special HTML token file.

Yet, upon creation of the *Service account* to be used by
``search-console``, such domains aren't automatically available
to the *Service account*: it needs to be manually registered from
the [Web Console Search][web-console-search], for each domain
(Settings, Users and permissions, +ADD USER), using the *Service
account* email address as an user identifier.

There are a few potential ways of automating this registration.
  - https://developers.google.com/site-verification;
  - OAuth on main user, listing of all domains, registration of
  all those domains on the service account using the [Search
  Console API][search-console-api].

# Plots @plots
[gonum/plot][gonum-plot] could be used to generate a ``.png``
file with stats, either to be send by [email][gh-mb-mmail] or
to be served by [httpd(8)][httpd-8].

# .json detection @json-detection
Specifying the path to the ``.json`` key file is a bit cumbersome;
we could chose a naming convention and/or drop it in a specific place
(e.g. ``/etc/search-console.json``).

# Man page/inline doc @doc

[gonum-plot]:         https://github.com/gonum/plot
[web-console-search]: https://search.google.com/search-console/
[search-console-api]: https://developers.google.com/webmaster-tools
[gh-mb-mmail]:        https://github.com/mbivert/mmail
[httpd-8]:            https://man.openbsd.org/httpd.8
