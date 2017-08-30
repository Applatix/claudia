'use strict';

const helpers = require('./helpers');
const webpackMerge = require('webpack-merge'); // used to merge webpack configs
const commonConfig = require('./webpack.common.js'); // the settings that are common to prod and dev

const DefinePlugin = require('webpack/lib/DefinePlugin');
const NamedModulesPlugin = require('webpack/lib/NamedModulesPlugin');
const LoaderOptionsPlugin = require('webpack/lib/LoaderOptionsPlugin');

const ENV = process.env.ENV = process.env.NODE_ENV = 'development';
const HOST = process.env.HOST || 'localhost';
const PORT = process.env.PORT || 3000;
const API_PROTOCOL = process.env.API_PROTOCOL || 'http://';
const API_URI = process.env.API_URI || 'localhost:3000/v1';
const HMR = helpers.hasProcessFlag('hot');
const PROXY = process.env.API_PROXY || 'https://ec2-35-165-177-51.us-west-2.compute.amazonaws.com:443';
const VERSION = process.env.VERSION || "development";

const METADATA = webpackMerge(commonConfig({ env: ENV }).metadata, {
    host: HOST,
    port: PORT,
    ENV: ENV,
    HMR: HMR,
    API_URI: API_URI,
    API_PROTOCOL: API_PROTOCOL,
    API_PROXY: PROXY,
    VERSION: VERSION,
});


module.exports = function () {
    return webpackMerge(commonConfig({ env: ENV }), {
        devtool: 'cheap-module-source-map',
        output: {
            path: helpers.root('dist'),
            filename: '[name].bundle.js',
            sourceMapFilename: '[name].map',
            chunkFilename: '[id].chunk.js',
            library: 'ac_[name]',
            libraryTarget: 'var',
        },

        plugins: [
            new DefinePlugin({
                'ENV': JSON.stringify(METADATA.ENV),
                'HMR': METADATA.HMR,
                'process.env': {
                    'ENV': JSON.stringify(METADATA.ENV),
                    'NODE_ENV': JSON.stringify(METADATA.ENV),
                    'HMR': METADATA.HMR,
                    'API_URI': JSON.stringify(METADATA.API_URI),
                    'API_PROTOCOL': JSON.stringify(METADATA.API_PROTOCOL),
                    "VERSION": JSON.stringify(METADATA.VERSION),
                },
            }),
            new NamedModulesPlugin(),
            new LoaderOptionsPlugin({
                debug: true,
                options: {
                    tslint: {
                        emitErrors: false,
                        failOnHint: false,
                        resourcePath: 'src'
                    },

                }
            }),

        ],

        devServer: {
            port: METADATA.port,
            host: METADATA.host,
            historyApiFallback: true,
            watchOptions: {
                aggregateTimeout: 300,
                poll: 1000
            },
            proxy: {

                '/v1': {
                    logLevel: 'debug',
                    target: METADATA.API_PROXY,
                    secure: false,
                    onError: function (err, req, res) {
                        console.error(err);
                    }
                },

            },
            outputPath: helpers.root('dist')
        },
        node: {
            global: true,
            crypto: 'empty',
            process: true,
            module: false,
            clearImmediate: false,
            setImmediate: false
        }
    });
};
