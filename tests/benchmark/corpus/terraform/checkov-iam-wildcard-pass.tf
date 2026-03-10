resource "aws_iam_policy" "pass2" {
  name = "pass2"
  path = "/"
  # deny
  policy = <<POLICY
{
  "Statement": [
    {
      "Action": "*",
      "Effect": "Deny",
      "Resource": "*",
      "Sid": ""
    }
  ],
  "Version": "2012-10-17"
}
POLICY
}
