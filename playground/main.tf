data "polaris_realm" "this" {}

output "realm" {
    value = data.polaris_realm.this.realm
}