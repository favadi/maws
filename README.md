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
# with homebrew
brew install favadi/maws/maws

# with go
go get github.com/favadi/maws
```

Or download the pre-built binaries in [releases
section](https://github.com/favadi/maws/releases).

Put following configuration to your shell configuration file
(~/.bashrc, ~/.zshrc):

```shell
# if you are using non-default aws profile
export MAWS_PROFILE=""

# enable completion, unnecessary if installed with homebrew
complete -C aws_completer maws
```

If you are using
[amazon-ecr-credential-helper](https://github.com/awslabs/amazon-ecr-credential-helper),
there is `docker-credential-mecr-login` wrapper script that use `maws`
instead of `aws` for MFA authentication. To use it, copy
`docker-credential-mecr-login` to somewhere in PATH (unnecessary if
installed with homebrew) and replace `ecr-login` in
`~/.docker/config.json` with `mecr-login`.

## Example usages

```shell
# first time, user will be prompted for OTP code
maws s3 ls

# second time, the token will be automatically loaded, user will not be prompted for OTP code
maws s3 ls

# a million of time later, the token is expired, user will be prompted for OTP code
maws s3 ls

# delete the persisted session token, the user will be prompted for OTP code next time
maws delete-session-token

# export environment variables to use persisted session token with normal aws command
eval $(maws export-envs)
```
