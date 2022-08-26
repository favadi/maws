# maws

maws is a wrapper of AWS CLI, that automatically authenticate the
current user with MFA and renew once the token is expired.

## Installation

Following
[this](https://aws.amazon.com/premiumsupport/knowledge-center/mfa-iam-user-aws-cli/)
to require MFA authentication for IAM users that use the AWS CLI.

Install the [AWS CLI version
2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).

Install maws:

```shell
go get github.com/favadi/maws
```

Put following configuration to your shell configuration file
(~/.bashrc, ~/.zshrc):

```shell
# if you are using non-default aws profile
export MAWS_PROFILE=""

# enable completion 
complete -C aws_completer maws
```

## Example usages

```shell
# first time, user will be prompted for OTP code
maws s3 ls

# second time, the token will be automatically loaded, user will not be prompted for OTP code
maws s3 ls

# a million of time later, the token is expired, user will be prompted for OTP code
maws s3 ls
```
