package config

import (
	"flag"
	"os" //used to communicate with operating system
	"log" //used to print log
 "github.com/ilyakaznacheev/cleanenv"
)

type HTTPServer struct {
	Addr string `yaml:"address" env:"HTTP_ADDRESS" env-default:"localhost:8080"`
}

type Config struct {
	Env         string `yaml:"env" env:"ENV" env-default:"production"`
	StoragePath string `yaml:"storage_path" env-default:"storage/storage.db" env-required:"true"`
	HTTPServer `yaml:"http_server"` //embedding is done here 
}

func MustLoad() *Config{
	var configPath string 

	configPath = os.Getenv("CONFIG_PATH")

	if configPath == ""{
		//flags : Used to read command-line arguments.
		flags := flag.String("config","","path to configuration file")
		flag.Parse() //Go  reads command-line arguments with this .
		
		configPath = *flags //flag.string return pointer 

		if configPath == ""{
			log.Fatal("Config path is not set") //fatal print message and immediately stops program
		}
	}

	if _,err := os.Stat(configPath); os.IsNotExist(err){ //Does this file exist and if not then give error 
		log.Fatalf("Config file does not exist: %s",configPath)
	}

var cfg Config

 err := cleanenv.ReadConfig(configPath, &cfg)
 if err != nil{
	log.Fatalf("Can not read config file: %s",err.Error())
 }


return &cfg

}

