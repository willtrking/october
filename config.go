package october

import (
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

const (
	modeEnvVariable         = "OCTOBER_MODE"
	portEnvVariable         = "OCTOBER_PORT"
	gqlPortEnvVariable      = "OCTOBER_GRAPHQL_PORT"
	grpcPortEnvVariable     = "OCTOBER_GRPC_PORT"
	tlsBundleCRTEnvVariable = "OCTOBER_TLS_BUNDLE_CRT"
	tlsKeyEnvVariable       = "OCTOBER_TLS_KEY"
	configuratorTagName     = "october"
)

// Generate a new configuratior, prefix may be an empty string
func NewEnvConfigurator() *Configurator {
	v := &Configurator{
		Viper: viper.New(),
	}
	return v
}

type Configurator struct {
	Viper *viper.Viper
}

func (c *Configurator) DecodeEnv(decodeInto interface{}, prefix string) error {

	c.Viper.SetEnvPrefix(strings.TrimSpace(prefix))

	keys := getTaggedConfigKeys(decodeInto)
	for _, key := range keys {
		c.Viper.BindEnv(key)
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           decodeInto,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
		TagName: configuratorTagName,
	})

	if err != nil {
		return err
	}

	return decoder.Decode(c.Viper.AllSettings())
}

func (c *Configurator) MustDecodeEnv(decodeInto interface{}, prefix string) {

	err := c.DecodeEnv(decodeInto, prefix)
	if err != nil {
		panic(err)
	}
}

func getTaggedConfigKeys(val interface{}) []string {
	valType := reflect.TypeOf(val)
	if valType.Kind() == reflect.Ptr {
		valType = valType.Elem()
	}

	var keys []string
	for i := 0; i < valType.NumField(); i++ {
		field := valType.Field(i)

		configKey := strings.TrimSpace(field.Tag.Get(configuratorTagName))
		if configKey != "" {
			keys = append(keys, configKey)
		}
	}

	return keys
}
