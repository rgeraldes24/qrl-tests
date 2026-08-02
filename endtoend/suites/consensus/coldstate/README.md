# Cold State Suite

Run this suite with the `cold` profile. It waits until genesis-era states are
beyond the configured archive interval, then retrieves and validates old
validator assignments through Qrysm's existing v1alpha1 API.
